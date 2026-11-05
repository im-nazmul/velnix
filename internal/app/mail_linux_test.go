//go:build linux

package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"velnix/internal/platform"
	"velnix/internal/providers/mailpit"
	"velnix/internal/providers/nativeexec"
	"velnix/internal/state"
)

func mailFixture(t *testing.T) MailService {
	t.Helper()
	base := t.TempDir()
	s := state.Store{Paths: platform.Paths{Data: filepath.Join(base, "data"), State: filepath.Join(base, "state")}}
	p, e := s.Plan("init")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Apply(context.Background(), "init", p.ID); e != nil {
		t.Fatal(e)
	}
	runtime := filepath.Join(base, "runtime")
	if e = os.Mkdir(runtime, 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("XDG_RUNTIME_DIR", runtime)
	return MailService{Store: s}
}
func TestMailAbsentReadsDoNotInitialize(t *testing.T) {
	root := t.TempDir()
	s := MailService{Store: state.Store{Paths: platform.Paths{Data: filepath.Join(root, "data"), State: filepath.Join(root, "state")}}}
	st, e := s.Status()
	if e != nil || st.Available {
		t.Fatalf("%+v %v", st, e)
	}
	if _, e = s.Read(context.Background(), MailRequest{Action: "list"}); e == nil {
		t.Fatal("missing capture silently started")
	}
	files, _ := os.ReadDir(root)
	if len(files) != 0 {
		t.Fatal("passive read created state")
	}
}
func TestMailPruneCandidates(t *testing.T) {
	now := time.Now()
	p := mailpit.Policy{MaxMessages: 2, MaxDays: 7}
	all := []mailpit.Summary{{ID: "older", Created: now.Add(-8 * 24 * time.Hour)}, {ID: "newest", Created: now}, {ID: "middle", Created: now.Add(-time.Hour)}, {ID: "excess", Created: now.Add(-2 * time.Hour)}}
	got := pruneCandidates(all, p, now)
	if strings.Join(got, ",") != "excess,older" {
		t.Fatal(got)
	}
}
func TestMailRealLifecycleExportAndPersistence(t *testing.T) {
	path := os.Getenv("VELNIX_TEST_MAILPIT")
	if path == "" {
		t.Skip("set VELNIX_TEST_MAILPIT to run disposable real-provider acceptance")
	}
	if os.Geteuid() == 0 {
		t.Skip("mail workloads require unprivileged user")
	}
	s := mailFixture(t)
	ctx := context.Background()
	req := MailRequest{Action: "serve", Executable: path, Policy: mailpit.Policy{SMTPPort: freePort(t), APIPort: freePort(t), MaxMessages: 500, MaxDays: 7}}
	// Even a hostile inherited environment must not configure the provider.
	t.Setenv("MP_UI_AUTH", "attacker:password")
	t.Setenv("MP_SMTP_RELAY_ALL", "true")
	t.Setenv("MP_WEBHOOK_URL", "http://127.0.0.1:1/leak")
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	p, e := s.Plan(ctx, req)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(s.Store.Paths.Data, "mail")); !os.IsNotExist(e) {
		t.Fatal("preview created mail store")
	}
	bad := req
	bad.Approval = "bad"
	if _, e = s.Apply(ctx, bad); e == nil {
		t.Fatal("incorrect approval accepted")
	}
	start := func(req MailRequest) (context.CancelFunc, chan error) {
		t.Helper()
		p, e := s.Plan(ctx, req)
		if e != nil {
			t.Fatal(e)
		}
		req.Approval = p.ID
		ready := make(chan struct{}, 1)
		s.Ready = func(MailRecord) error { ready <- struct{}{}; return nil }
		run, cancel := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() { _, e := s.Apply(run, req); done <- e }()
		select {
		case <-ready:
		case e := <-done:
			cancel()
			t.Fatalf("mail early exit: %v", e)
		case <-time.After(8 * time.Second):
			cancel()
			t.Fatal("mail readiness timed out")
		}
		t.Cleanup(func() { cancel() })
		return cancel, done
	}
	stop := func(cancel context.CancelFunc, done chan error) {
		t.Helper()
		cancel()
		select {
		case e := <-done:
			if e != nil && !errors.Is(e, context.Canceled) {
				t.Fatal(e)
			}
		case <-time.After(8 * time.Second):
			t.Fatal("mail did not stop")
		}
	}
	cancel, done := start(req)
	defer cancel()
	if _, e = s.Apply(ctx, MailRequest{Action: "serve", Executable: path, Policy: req.Policy, Approval: p.ID}); e == nil {
		t.Fatal("concurrent capture accepted")
	}
	smtpAddress := fmt.Sprintf("127.0.0.1:%d", req.Policy.SMTPPort)
	raw := "From: sender@example.test\r\nTo: recipient@example.test\r\nSubject: hello\x1b[31m\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=parts\r\n\r\n--parts\r\nContent-Type: text/plain\r\n\r\nplain body\r\n--parts\r\nContent-Type: application/octet-stream\r\nContent-Disposition: attachment; filename=\"../../escape.bin\"\r\nContent-Transfer-Encoding: base64\r\n\r\nAAH/\r\n--parts--\r\n"
	if e = smtp.SendMail(smtpAddress, nil, "sender@example.test", []string{"recipient@example.test"}, []byte(raw)); e != nil {
		t.Fatal(e)
	}
	v, e := s.Read(ctx, MailRequest{Action: "list"})
	if e != nil {
		t.Fatal(e)
	}
	page := v.(mailpit.Page)
	if page.Total != 1 {
		t.Fatalf("%+v", page)
	}
	id := page.Messages[0].ID
	if strings.Contains(page.Messages[0].Subject, "\x1b") {
		t.Fatal("terminal escape exposed")
	}
	v, e = s.Read(ctx, MailRequest{Action: "show", ID: id})
	if e != nil {
		t.Fatal(e)
	}
	msg := v.(mailpit.Message)
	if !strings.Contains(msg.Text, "plain body") || len(msg.Attachments) != 1 || msg.Attachments[0].Name != "../../escape.bin" {
		t.Fatalf("%+v", msg)
	}
	// Raw inspection must not mark the message read.
	c, _, e := s.client()
	if e != nil {
		t.Fatal(e)
	}
	afterRead, listErr := c.List(ctx, 0)
	if listErr != nil || afterRead.Messages[0].Read {
		t.Fatal("inspection mutated read status", listErr)
	}
	b, e := c.Raw(ctx, id)
	c.Close()
	if e != nil || len(b) == 0 {
		t.Fatal(e)
	}
	output := filepath.Join(t.TempDir(), "attachment.bin")
	export := MailRequest{Action: "export", ID: id, Attachment: msg.Attachments[0].ID, Output: output}
	ep, e := s.Plan(ctx, export)
	if e != nil {
		t.Fatal(e)
	}
	export.Approval = ep.ID
	if _, e = s.Apply(ctx, export); e != nil {
		t.Fatal(e)
	}
	bytes, e := os.ReadFile(output)
	if e != nil || string(bytes) != string([]byte{0, 1, 255}) {
		t.Fatalf("attachment changed: %v", e)
	}
	info, _ := os.Stat(output)
	if info.Mode().Perm() != 0600 {
		t.Fatal("unsafe exported permissions")
	}
	if _, e = s.Apply(ctx, export); e == nil {
		t.Fatal("export overwrote existing destination")
	}
	// Export plan expires when the destination appears.
	stale := MailRequest{Action: "export", ID: id, Attachment: "1", Output: filepath.Join(t.TempDir(), "stale")}
	sp, e := s.Plan(ctx, stale)
	if e != nil {
		t.Fatal(e)
	}
	os.WriteFile(stale.Output, []byte("external"), 0600)
	stale.Approval = sp.ID
	if _, e = s.Apply(ctx, stale); e == nil {
		t.Fatal("stale export applied")
	}
	prune := MailRequest{Action: "prune"}
	pp, e := s.Plan(ctx, prune)
	if e != nil || len(pp.Candidates) != 0 {
		t.Fatalf("unexpected prune %+v %v", pp, e)
	}
	prune.Approval = pp.ID
	// New mail invalidates the exact snapshot even if candidates remain empty.
	if e = smtp.SendMail(smtpAddress, nil, "sender@example.test", []string{"recipient@example.test"}, []byte("From: sender@example.test\r\nTo: recipient@example.test\r\nSubject: two\r\n\r\ntwo")); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Apply(ctx, prune); e == nil {
		t.Fatal("stale prune applied")
	}
	pp, e = s.Plan(ctx, MailRequest{Action: "prune"})
	if e != nil {
		t.Fatal(e)
	}
	prune.Approval = pp.ID
	if _, e = s.Apply(ctx, prune); e != nil {
		t.Fatal(e)
	}
	st, e := s.Status()
	if e != nil || !st.Available {
		t.Fatalf("%+v %v", st, e)
	}
	session := st.Record.Session
	if _, e = s.Store.MailDisk(); e != nil {
		t.Fatal(e)
	}
	stop(cancel, done)
	if _, e = os.Stat(filepath.Join(session, "credentials.json")); !os.IsNotExist(e) {
		t.Fatal("credentials survived stop")
	}
	for _, port := range []int{req.Policy.SMTPPort, req.Policy.APIPort} {
		listener, e := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", port))
		if e != nil {
			t.Fatal("listener survived stop", e)
		}
		listener.Close()
	}
	if _, e = s.Read(ctx, MailRequest{Action: "list"}); e == nil {
		t.Fatal("stopped read restarted capture")
	}
	cancel, done = start(req)
	v, e = s.Read(ctx, MailRequest{Action: "list"})
	if e != nil || v.(mailpit.Page).Total != 2 {
		t.Fatalf("restart persistence: %+v %v", v, e)
	}
	c, _, e = s.client()
	if e != nil {
		t.Fatal(e)
	}
	all, e := c.All(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if e = c.Delete(ctx, []string{all[0].ID}); e != nil {
		t.Fatal(e)
	}
	all, e = c.All(ctx)
	c.Close()
	if e != nil || len(all) != 1 {
		t.Fatalf("exact provider deletion failed: %v %v", all, e)
	}
	// Restore a second message for the automatic count-retention check.
	if e = smtp.SendMail(smtpAddress, nil, "sender@example.test", []string{"recipient@example.test"}, []byte("From: sender@example.test\r\nTo: recipient@example.test\r\nSubject: retained\r\n\r\nretained")); e != nil {
		t.Fatal(e)
	}
	stop(cancel, done)
	req.Policy.MaxMessages = 1
	cancel, done = start(req)
	deadline := time.Now().Add(70 * time.Second)
	for {
		value, err := s.Read(ctx, MailRequest{Action: "list"})
		if err != nil {
			t.Fatal(err)
		}
		if value.(mailpit.Page).Total == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("provider count retention did not converge")
		}
		time.Sleep(200 * time.Millisecond)
	}
	stop(cancel, done)
	// Seed an old received timestamp only in a disposable provider fixture.
	// Production capture never enables --use-message-dates.
	selection, err := mailpit.Select(path)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := nativeexec.Open(selection)
	if err != nil {
		t.Fatal(err)
	}
	defer provider.Close()
	disk, err := s.Store.MailData(false)
	if err != nil {
		t.Fatal(err)
	}
	defer disk.Close()
	credentials := mailpit.Credentials{User: "fixture", Password: strings.Repeat("a", 64)}
	policy := req.Policy
	policy.MaxMessages = 500
	command := mailpit.Command(provider, disk.File(), selection, policy, credentials, os.Getenv("XDG_RUNTIME_DIR"))
	command.Args = append(command.Args, "--use-message-dates")
	if err = command.Start(); err != nil {
		t.Fatal(err)
	}
	fixtureDone := make(chan error, 1)
	go func() { fixtureDone <- command.Wait() }()
	t.Cleanup(func() { _ = command.Process.Kill() })
	fixtureClient, err := mailpit.NewClient(policy.APIPort, credentials)
	if err != nil {
		t.Fatal(err)
	}
	defer fixtureClient.Close()
	fixtureReady := false
	for until := time.Now().Add(5 * time.Second); time.Now().Before(until); {
		if _, err = fixtureClient.List(ctx, 0); err == nil {
			fixtureReady = true
			break
		}
		time.Sleep(40 * time.Millisecond)
	}
	if !fixtureReady {
		t.Fatal("age fixture not ready")
	}
	oldDate := time.Now().Add(-8 * 24 * time.Hour).Format(time.RFC1123Z)
	oldRaw := "From: sender@example.test\r\nTo: recipient@example.test\r\nSubject: expired\r\nDate: " + oldDate + "\r\n\r\nold"
	if err = smtp.SendMail(smtpAddress, nil, "sender@example.test", []string{"recipient@example.test"}, []byte(oldRaw)); err != nil {
		t.Fatal(err)
	}
	oldPage, err := fixtureClient.List(ctx, 0)
	if err != nil || oldPage.Total != 2 {
		t.Fatalf("age fixture capture: %v %v", oldPage, err)
	}
	hasOld := false
	for _, m := range oldPage.Messages {
		hasOld = hasOld || m.Created.Before(time.Now().Add(-7*24*time.Hour))
	}
	if !hasOld {
		t.Fatal("fixture did not preserve old received timestamp")
	}
	_ = command.Process.Signal(os.Interrupt)
	select {
	case <-fixtureDone:
	case <-time.After(5 * time.Second):
		t.Fatal("age fixture failed to stop")
	}
	req.Policy.MaxMessages = 500
	cancel, done = start(req)
	deadline = time.Now().Add(70 * time.Second)
	for {
		value, err := s.Read(ctx, MailRequest{Action: "list"})
		if err != nil {
			t.Fatal(err)
		}
		if value.(mailpit.Page).Total == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("provider age retention did not converge")
		}
		time.Sleep(200 * time.Millisecond)
	}
	// A write failure preserves an unresolved operation, blocking subsequent
	// mutations until the capture is stopped and recovery is explicitly reviewed.
	value, err := s.Read(ctx, MailRequest{Action: "list"})
	if err != nil {
		t.Fatal(err)
	}
	remainingID := value.(mailpit.Page).Messages[0].ID
	// Export failure is exercised using a binary attachment message.
	if err = smtp.SendMail(smtpAddress, nil, "sender@example.test", []string{"recipient@example.test"}, []byte(raw)); err != nil {
		t.Fatal(err)
	}
	value, err = s.Read(ctx, MailRequest{Action: "list"})
	if err != nil {
		t.Fatal(err)
	}
	var attachmentID string
	for _, m := range value.(mailpit.Page).Messages {
		if m.ID != remainingID {
			attachmentID = m.ID
		}
	}
	deniedParent := t.TempDir()
	denied := MailRequest{Action: "export", ID: attachmentID, Attachment: "1", Output: filepath.Join(deniedParent, "denied")}
	dp, err := s.Plan(ctx, denied)
	if err != nil {
		t.Fatal(err)
	}
	denied.Approval = dp.ID
	if err = os.Chmod(deniedParent, 0500); err != nil {
		t.Fatal(err)
	}
	_, err = s.Apply(ctx, denied)
	os.Chmod(deniedParent, 0700)
	if err == nil {
		t.Fatal("write failure not surfaced")
	}
	unresolved, err := s.Status()
	if err != nil || !unresolved.RecoveryRequired || unresolved.LastOperation.Phase != "recovery-required" {
		t.Fatalf("missing recovery evidence %+v %v", unresolved, err)
	}
	if _, err = s.Plan(ctx, MailRequest{Action: "prune"}); err == nil {
		t.Fatal("unresolved evidence overwritten")
	}
	stop(cancel, done)
	recoverReq := MailRequest{Action: "recover"}
	rp, err := s.Plan(ctx, recoverReq)
	if err != nil {
		t.Fatal(err)
	}
	recoverReq.Approval = rp.ID
	if _, err = s.Apply(ctx, recoverReq); err != nil {
		t.Fatal(err)
	}
	if evidence, err := s.Store.ReadWorkload("mail-recovery-evidence.json"); err != nil || len(evidence) == 0 {
		t.Fatal("interrupted evidence lost", err)
	}
	// Ensure process/evidence output never stores private API passwords.
	op, e := s.Store.ReadWorkload("mail-operation.json")
	if e != nil {
		t.Fatal(e)
	}
	var operation MailOperation
	if e = json.Unmarshal(op, &operation); e != nil || operation.Phase != "complete" {
		t.Fatalf("%s %v", op, e)
	}
}
