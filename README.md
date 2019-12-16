# jsso - jrockway's single sign on

(This document, and the project itself, are in the very very early stages. Things that are mentioned
as though they exist do not. Star the repository and I'll keep you updated! This is the second time
I've written an SSO system and this time it's going to be perfect ;)

This project arose from the beginnings of my time with Kubernetes. I had this fancy new cluster,
and I could finally run things like Prometheus and Jaeger and really see what was going on with my
software. One small problem, though, was that these apps displayed information that I wanted to
keep inside my company, but they didn't have any built-in way of authenticating a user. I struggled
for a while with an OpenVPN installation and ClusterIPs in DNS, and it was ... okay. But eventually
we started writing our own apps that needed authentication, and wanted "normal people" to be able to
access them without having to issue certificates and install a VPN for them. Furthermore, we
already had too many single sign on systems. The root of trust was Okta, and some internal
applications just used that directly. It was a long and complicated process to add another "app",
though, and I knew I would be too lazy to ever do that. And they charged you money for it. Other
teams used a Duo proxy, and the way it worked often required you to sign in to an application twice;
once to tell Duo who you were so that the proxy would pass your traffic, and then once to sign in to
the actual app. The proxy didn't tell the app who you were. The apps that did have authentication
built in caused another problem; you had to sign into every single one. The end result is that a
even small company had three single sign on providers, and you had to sign in to every app. That is
not single sign on, that is encouragement to not use any apps, because you have to go through so
many steps, or to not write any more apps, because you have to go through even more steps to set one
up with authentication.

I looked around for options at the time. buzzfeed-sso looked almost all right, but it had some
weird technical requirements that didn't work for us. I ultimately decided on bitly's oauth2_proxy.
It at least let one access something internally, but still required two Kubernetes configuration
files and a trip to Google's API console for every app you wanted to add. And it was marked
deprecated. So I knew I had to do something about that. And did.

jsso is the evolution of that system.

The goal of the jsso project is to let you take back control over authentication. Users should only
have to sign in once per day per computer. Developers should not have to do anything to launch a
new service.

## Architectural overview

The idea behind jsso is to provide the parts that work the way you want while letting you easily
write the parts you don't. That way, you don't have to read this, see that some detail annoys you
enough to not use it, and be sad about not having authentication. You can keep the parts you like
and rewrite the part you don't, and still have a working system. No compromises, no extra work.

Here's how we accomplish that.

jsso is divided into a data plane and a control plane. The data plane looks at network requests and
decides whether or not they are allowed, and if they are allowed, how they should interact with
upstream systems. The control plane decides who the user is and what policies grant or restrict
access.

### The data plane

The data plane is simplest. It consits of a program that runs next to your web server which
extracts information from a request, tries to get as much information about the request as possible,
and then passes all of that to Open Policy Agent to decide how to proceed.

Extracting information from the request is the interesting part. Users might have a session cookie;
we send that up to the control plane to turn it into a username. Session cookies might not be
adequate for every request, so we provide a variety of ways to extract a piece of authentication
information. (For example; basic auth, a JWT, or a bearer token. Cookies are how humans using a
web brower authenticate themselves. Basic auth is how Zendesk authenticates itself to your webhook
endpoints. A special JWT is how Dialpad authenticates itself to your webhook. A bearer token is
how an API user, like an automated process on an external system, might authenticate. And, there
are of course TLS client certificates. The idea is that we want to care about this for the shortest
amount of time possible; ask the control plane to turn ANY credential into something that a policy
can take action on.)

Using Open Policy Agent on the request, after we've asked the control plane to "fill in the blanks",
lets you write unit-testable and hot-reloadable configuration for every route. You can restrict
access to certain IPs. You can say "this is public, I don't care about the user". Really,
anything! Finally, the policy can decide what this request looks like to the upstream application.
You can pass it a token good for one request, so the upstream application can trust that the request
really is legitimate without having to call out to other services. You can just pass the username
in a header (some apps want this). You can convert the user's cookie into a bearer token. The idea
is, you can take any input that maps to an authenticated user, and produce any output that will make
the upstream application happy. You can make weird external clients make internal requests without
a special proxy just for them. And you can make weird third-party apps happy, and not have to make
your users sign in a second time because they're weird.

### The control plane

There are several components on the control plane.

The first is a system for allowing unknown users to identify themselves. This is the good-old "user
table" or maybe an external service, like Okta or Gsuites. The idea is that you provide an app that
decides who a user is (perhaps asking them for a password or for them to touch their Webauthn
token). If satisfied, that app then tells the "session service" to mint a new session. The "session
service" creates a session and sends your app a "set cookie token" for setting cookies, and a URL
where the user should go to to have that done. (The user service can also tell the sesion service to
revoke a user's sessions, for logouts, account takeover, etc.)

The "session service" just maps session IDs to usernames. This is an extra service because the
implementation really depends on how many requests you're seeing; maybe you want to use your MySQL
database, maybe you want to use Redis. You can scale it up or down depending on your needs.

The "set cookie" service is what lets you really have a "single" sign on in your organization. You
might have multiple domain names, but cookies only stick to one. The "set cookie" service walks your
user's browser through all of your domain names with a cryptographically authenticated set-cookie
token, so that signing into "example.com" also signs them into "example.net". This was the feature
that actually drove me to write my own system. At my last company, we had customer-facing
production applications on .com, internal production appliations on .ninja, and staging on .dog.
Internal users needed to be able to use all three domains, and I didn't want to make them sign in
three times. (It would just mean that nobody would ever use the internal apps or send us feedback
about staging versions. ;)

The last component is the policy and audit service. This pushes out policy updates to the data
plane, and receives audit events from the data plane. You might be satisifed with policies that
never change, and with audits in the form of log files, so you might not need this service. This is
also the part where you'd add a "web application firewall".

### Extras

This is all fine for HTTP requests that need to originate from outside your production environment,
but what about integrating with the outside world?

The plan is to just write services that you can run to get these features. SAML and OpenID Connect
can just be a normal web app whose URL happens to go through the SSO flow first. LDAP and RADIUS
are just plugins for your user service.

The control plane can also be extended, so that users can mint "robot accounts" or "api keys" for
non-human users (like that big screen that shows dashboards; or did until people decided it wasn't
worth the effort to create an account and log into it every day). I am also playing with the idea
of links that authorize exactly one HTTP session, so you can click a button in your browser and get
a shareable link. Both need deep integration with the control plane; your policy can allow users to
make these one-time links to Grafana dashboards, but you probably don't want them to send a link to
your sales leads.
