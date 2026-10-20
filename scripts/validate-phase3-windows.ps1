# Opt-in read-only Windows-to-WSL TLS check. Root input is PUBLIC DER, never a key.
# No hosts/store imports, browser automation, network reconfiguration or downloads.
param(
    [Parameter(Mandatory=$true)][ValidateRange(1024,65535)][int]$Port,
    [Parameter(Mandatory=$true)][ValidatePattern('^[a-z0-9][a-z0-9-]*\.test$')][string]$Name,
    [Parameter(Mandatory=$true)][string]$RootCertificateBase64,
    [Parameter(Mandatory=$true)][ValidatePattern('^[a-f0-9]{64}$')][string]$LeafSHA256
)
$ErrorActionPreference = 'Stop'
Add-Type -TypeDefinition @'
using System;
using System.IO;
using System.Net.Security;
using System.Net.Sockets;
using System.Security.Authentication;
using System.Security.Cryptography;
using System.Security.Cryptography.X509Certificates;
using System.Text;

public static class VelnixPhase3TlsAcceptance {
    static TcpClient Connect(int port) {
        var client = new TcpClient();
        var pending = client.BeginConnect("127.0.0.1", port, null, null);
        if (!pending.AsyncWaitHandle.WaitOne(5000)) {
            client.Close(); throw new IOException("Windows loopback connection timed out");
        }
        client.EndConnect(pending); client.ReceiveTimeout = 5000; client.SendTimeout = 5000;
        return client;
    }
    static string Digest(byte[] bytes) {
        using (var sha = SHA256.Create()) {
            return BitConverter.ToString(sha.ComputeHash(bytes)).Replace("-", "").ToLowerInvariant();
        }
    }
    public static bool DefaultTrust(int port, string name) {
        using (var client = Connect(port))
        using (var stream = new SslStream(client.GetStream(), false,
            (sender, cert, chain, errors) => errors == SslPolicyErrors.None)) {
            try { stream.AuthenticateAsClient(name, null, SslProtocols.Tls12, false); return true; }
            catch (AuthenticationException) { return false; }
        }
    }
    public static string ExplicitCA(int port, string name, string rootBase64, string leafDigest) {
        using (var root = new X509Certificate2(Convert.FromBase64String(rootBase64)))
        using (var client = Connect(port))
        using (var stream = new SslStream(client.GetStream(), false, (sender, cert, suppliedChain, errors) => {
            if ((errors & (SslPolicyErrors.RemoteCertificateNameMismatch | SslPolicyErrors.RemoteCertificateNotAvailable)) != 0)
                return false;
            if (cert == null || Digest(cert.GetRawCertData()) != leafDigest) return false;
            using (var chain = new X509Chain()) {
                chain.ChainPolicy.ExtraStore.Add(root);
                chain.ChainPolicy.VerificationFlags = X509VerificationFlags.AllowUnknownCertificateAuthority;
                chain.ChainPolicy.RevocationMode = X509RevocationMode.NoCheck;
                if (!chain.Build(new X509Certificate2(cert))) return false;
                var anchor = chain.ChainElements[chain.ChainElements.Count - 1].Certificate;
                return Digest(anchor.RawData) == Digest(root.RawData);
            }
        })) {
            stream.AuthenticateAsClient(name, null, SslProtocols.Tls12, false);
            var request = Encoding.ASCII.GetBytes("GET / HTTP/1.1\r\nHost: " + name + "\r\nConnection: close\r\n\r\n");
            stream.Write(request, 0, request.Length); stream.Flush();
            using (var reader = new StreamReader(stream, Encoding.ASCII)) {
                var status = reader.ReadLine();
                if (status == null || !status.Contains(" 200 ")) throw new IOException("Unexpected HTTP response: " + status);
                return status;
            }
        }
    }
}
'@
$before = [VelnixPhase3TlsAcceptance]::DefaultTrust($Port, $Name)
$status = [VelnixPhase3TlsAcceptance]::ExplicitCA($Port, $Name, $RootCertificateBase64, $LeafSHA256)
$after = [VelnixPhase3TlsAcceptance]::DefaultTrust($Port, $Name)
if ($before -or $after) { throw 'Disposable CA was unexpectedly trusted by default' }
[ordered]@{
    windows_loopback = $true
    explicit_ca = $true
    hostname_and_leaf_verified = $true
    default_trust_before = $before
    default_trust_after = $after
    http_status = $status
    windows_browser_trust = 'unverified; separate manual browser/store acceptance required'
    hosts_resolution = 'not exercised; direct loopback transport with exact TLS name'
} | ConvertTo-Json -Compress
