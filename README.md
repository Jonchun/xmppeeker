
# XMPPeeker
## Introduction
XMPPeeker is a tool created as a side project that may be useful for specific debugging situations. The scenario I was trying to solve was that in a production deployment, it is not feasible to log all XMPP stanzas due to the heavy IO burden it places on the system. The product I was working on did not have functionality to log only a subset of users and it was difficult/impossible to troubleshoot certain problems since we didn't have access to the XMPP stanzas being exchanged. tcpdump didn't help either because the XMPP server in our product required TLS.

The workaround I came up with was a Man-In-The-Middle (MITM) proxy that could interpret the XMPP protocol and negotiate XMPP over TLS on two separate connections, then bridge the two encrypted connections by forwarding the unencrypted contents of each TCP stream to the other.

## Installation
TODO


## Configuration
XMPPeeker has a simple configuration and deployment. The only required configuration field is `BackendHost`, which is the XMPP server you want to reverse-proxy.


## Usage
Any clients you want to peek at XMPP traffic for should now connect to the configured `ListenHost` instead of the original `BackendHost`.

Below is a simple diagram illustrating the connections. 
```
Client (C) <-> Proxy/XMPPeeker (P) <-> Server (S)
```
XMPPeeker transparently overwrites any XMPP stream attributes, so the Client thinks that it is talking to the Server directly and vice versa.

Logs are generated for each unique client connection to XMPPeeker. There are 2 types of logs.
```
C2P: Client <-> Proxy | logs traffic between Client and Proxy
P2S: Proxy <-> Server | logs -traffic between Proxy and Server
```
They are created in the following format:  `$LogPath/$ClientIP/$Timestamp.$Type.log`. 

Currently, XMPPeeker watches the XMPP stream for the SASL success message
```
<success xmlns="urn:ietf:params:xml:ns:xmpp-sasl"></success>
```
After a SASL success is identified, XMPPeeker stops parsing the streams as XML since the XMPP stream is fully open and doesn't need to be restarted again and it simply does a byte-level copy between the two streams (while still logging every read/write on each connection)


## Known Issues
While the majority of the contents of the `C2P` should match the contents of the `P2S` logs, there are occasional minor differences between the two files (which are easily identifiable by a human as equivalent/not a problem) which are artifacts of how the transparent proxy process was implemented.

In the following example, 
```
BackendHost=xmppeeker.backend.lan
ListenHost=xmppeeker.proxy.lan
```

**C2P**
```
2021-08-01 19:58:06.400938 C->P <stream:stream to="xmppeeker.proxy.lan" xmlns="jabber:client" xmlns:stream="http://etherx.jabber.org/streams" version="1.0">
2021-08-01 19:58:06.477944 P->C <stream:stream xmlns="jabber:client" xmlns:stream="http://etherx.jabber.org/streams" from="xmppeeker.backend.lan" version="1.0" id="17dd95bab6ad4147a34863b6e59a6912">
2021-08-01 19:58:06.478036 P->C <stream:features xmlns:stream="http://etherx.jabber.org/streams"><starttls xmlns="urn:ietf:params:xml:ns:xmpp-tls"><required></required></starttls></stream:features>
2021-08-01 19:58:06.478898 C->P <starttls xmlns="urn:ietf:params:xml:ns:xmpp-tls"/>
2021-08-01 19:58:06.815886 P->C <proceed xmlns="urn:ietf:params:xml:ns:xmpp-tls"></proceed>
```
**P2S**
```
2021-08-01 19:58:06.401116 P->S <stream:stream to="xmppeeker.backend.lan" xmlns="jabber:client" xmlns:stream="http://etherx.jabber.org/streams" version="1.0">
2021-08-01 19:58:06.477780 S->P <?xml version="1.0" encoding="UTF-8"?><stream:stream xmlns="jabber:client" xmlns:stream="http://etherx.jabber.org/streams" from="xmppeeker.backend.lan" version="1.0" id="17dd95bab6ad4147a34863b6e59a6912"><stream:features xmlns:stream="http://etherx.jabber.org/streams"><starttls xmlns="urn:ietf:params:xml:ns:xmpp-tls"><required></required></starttls></stream:features>
2021-08-01 19:58:06.478971 P->S <starttls xmlns="urn:ietf:params:xml:ns:xmpp-tls"></starttls>
2021-08-01 19:58:06.556305 S->P <proceed xmlns="urn:ietf:params:xml:ns:xmpp-tls"></proceed>
```
First, the obvious/expected differences exist on L1 of both files. Note that `stream to="xmppeeker.proxy.lan"` gets overwritten to `stream to="xmppeeker.backend.lan"`

Next, note how there is an extremely minor variation in what the server sends to the proxy on `P2S, L2` compared to what the proxy sends to the client `C2P, L2-3`. The Client gets sent the `stream:stream` and `stream:features` elements separately. These minor variations occur because XMPPeeker needs to parse and understand the XMPP protocol in order to properly negotiate TLS. None of these discrepancies should affect functionality, and because both log types are written, any differences can easily be analyzed.

## License
[MIT](LICENSE)
