import hashlib,json,os,pathlib,socket,subprocess,tempfile,time
# Developer-only acceptance test. Requires Python 3 and installed Nginx.
# Usage: python3 scripts/smoke-http.py /absolute/path/to/velnix
import sys
if len(sys.argv)!=2:
 raise SystemExit('usage: python3 scripts/smoke-http.py /absolute/path/to/velnix')
binary=str(pathlib.Path(sys.argv[1]).resolve())
def snapshot():
 return {str(p):hashlib.sha256(p.read_bytes()).hexdigest() for p in pathlib.Path('/etc/nginx').rglob('*') if p.is_file()}
before=snapshot()
with tempfile.TemporaryDirectory(prefix='velnix-phase2-cli-') as folder:
 env=dict(os.environ,XDG_DATA_HOME=folder+'/data',XDG_STATE_HOME=folder+'/state',XDG_CONFIG_HOME=folder+'/config')
 def cli(*args):
  result=subprocess.run([binary,*args,'--json'],env=env,capture_output=True,text=True,timeout=10)
  assert result.returncode==0,(args,result.returncode,result.stderr)
  return json.loads(result.stdout)
 def apply(*args):
  p=cli(*args,'--dry-run');return cli(*args,'--approve',p['id'])
 apply('state','init')
 root=pathlib.Path(folder)/'site';root.mkdir();(root/'index.html').write_text('cli smoke passed')
 apply('projects','link','--name','smoke','--path',str(root))
 with socket.socket() as sock:
  sock.bind(('127.0.0.1',0));port=sock.getsockname()[1]
 args=['http','start','--port',str(port)]
 plan=cli(*args,'--dry-run')
 child=subprocess.Popen([binary,*args,'--approve',plan['id'],'--json'],env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
 try:
  for attempt in range(100):
   status=cli('http','status')
   if status['record'] and status['record']['phase']=='running':break
   assert child.poll() is None,child.communicate()
   time.sleep(.03)
  else:raise AssertionError('not ready')
  import http.client
  conn=http.client.HTTPConnection('127.0.0.1',port,timeout=2);conn.request('GET','/',headers={'Host':'smoke.test'});response=conn.getresponse();assert response.status==200;assert response.read()==b'cli smoke passed';conn.close()
  apply('http','stop')
  out,err=child.communicate(timeout=8);assert child.returncode==0,(child.returncode,err);assert json.loads(out)['record']['phase']=='stopped'
  assert not cli('http','status')['alive']
  apply('projects','unlink','--name','smoke');assert (root/'index.html').read_text()=='cli smoke passed'
  with socket.socket() as sock:
   sock.settimeout(.2);assert sock.connect_ex(('127.0.0.1',port))!=0
 finally:
  if child.poll() is None:child.terminate();child.communicate(timeout=8)
assert snapshot()==before
print(json.dumps({'compiled_cli_smoke':'passed','external_nginx_files_unchanged':len(before),'owned_listener_closed':True}))
