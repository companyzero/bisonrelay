package main

const (
	defaultConfigFileContent = `

# address of the server
server = {{ .ServerAddr }}

# root directory for brclient settings, db, etc
root = {{ .Root }}

# launch windows for DMs and GCs on startup
# winpin = user1,user2,gc1,gc2,gc3

# Compression level for messages sent.
# 0=no compression, 9=best compression (slowest).
# compresslevel = 4

# Proxy Configuration. Also needed for accessing the server as a TOR hidden
# service.
# proxyaddr =
# proxyuser =
# proxypass =
# torisolation = 0
# circuitlimit = 32

# external viewer for mimetypes
# mimetype=image/*,ristretto
# mimetype=video/*,mplayer
# mimetype=text/html,firefox
# mimetype=text/*,vi

# Bell Command: executed on incoming msgs. *BEEP* outputs the terminal BEL.
# In arguments, '$src' is replaced with the alias of the sender or GC. '$msg' is
# replaced with the message. Some examples.
#
# Ring the terminal BEL.
# bellcmd = *BEEP*
#
# Show a desktop notification.
# bellcmd = notify-send -i mail-unread "[$src]> $msg"

# logging and debug
[log]

# Where the message logs are stored. Set to an empty value to not save msg logs.
msglog = {{ .MsgRoot }}

# logfile contains log file name location
logfile = {{ .LogFile }}

# How many log files to keep about the internal operations. 0 means keep all
# log files.
maxlogfiles = 0

# how verbose to be
debuglevel = info

# Whether to save command history to a file.
savehistory = false

# Valid ui colors: na, black, red, green, yellow, blue, magenta, cyan and white
# Valid attributes are: none, underline and bold
# format is: attribute:foreground:background
[theme]
nickcolor = bold:na:na
gcothercolor = bold:green:na
pmothercolor = bold:cyan:na
blinkcursor = true


[payment]

# Type of ln wallet to use. Either "internal" (for an embedded wallet),
# "external" (to connect to an already running LN wallet) or "disabled" to
# disable payments (server must support sending msgs for free).
wallettype = {{ .WalletType }}

# The next parameters are set when using an internal (embedded) LN wallet,
# otherwise they are commented out.

# Network is the network to use to initialize the internal wallet (mainnet,
# testnet, simnet).
{{ if eq .WalletType "internal" -}}
network = {{ .Network }}
{{ else -}}
# network = mainnet
{{ end }}

# The next parameters are set when connecting to an external wallet. Otherwise
# they are commented out.

# Host of an the external dcrlnd instance
{{ if eq .WalletType "external" -}}
lnrpchost = {{ .LNRPCHost }}
{{ else -}}
# lnrpchost = 127.0.0.1:10009
{{ end }}

# Cert path of the dcrlnd instance
{{ if eq .WalletType "external" -}}
lntlscert = {{ .LNTLSCertPath }}
{{ else -}}
# lntlscert = ~/.dcrlnd/tls.cert
{{ end }}

# Path to a valid macaroon file. Replace 'mainnet' with 'testnet' if needed.
{{ if eq .WalletType "external" -}}
lnmacaroonpath = {{ .LNMacaroonPath }}
{{ else -}}
# lnmacaroonpath = ~/.dcrlnd/data/chain/decred/mainnet/admin.macaroon
{{ end }}

# Log Level of the internal dcrlnd
# lndebuglevel = info

# Max nb of log files for LN logs
# lnmaxlogfiles = 3

# Minimum balances specified in DCR which will trigger a warning prompt
# to deposit more funds.
minimumwalletbalance = 1.0
minimumrecvbalance = 0.01
minimumsendbalance = 0.01

# LN RPC listen addresses. Only used with internal dcrlnd instance. Comma
# separated. If specified, the first address MUST be a locally accessible one
# (such as 127.0.0.1:10009).
# lnrpclisten = 127.0.0.1:<port>

# Account to use to generate private keys and store funds to send to remote
# users on-chain inside invites.
# invitefundsaccount = non-default-account

[clientrpc]
# Enable the JSON-RPC clientrpc protocol on the comma-separated list of addresses.
# jsonrpclisten = 127.0.0.1:7676

# Path to the keypair used for running TLS on the clientrpc interfaces.
# rpccertpath = {{ .Root }}/rpc.cert
# rpckeypath = {{ .Root }}/rpc.key

# Path to the certificate used as CA for client-side TLS authentication.
# rpcclientcapath = {{ .Root }}/rpc-ca.cert

# If set to true, generate the rpc-client.cert and rpc-client.key files in the
# same dir as rpcclientcapath, that should be specified by a client connecting
# over the clientrpc interfaces. If set to false, then the user is responsible
# for generating the client CA, and cert files.
# rpcissueclientcert = true

[resources]
# Use an upstream processor for handling resource requests. Options:
# "pages:<path>" offers static pages stored in the local <path>.
# "simplestore:<path>" uses the internal 'simplestore' subsystem; if <path> does
#   not exist, then it will be created and fill with a sample, minimal store.
# "clientrpc": sends request events and waits for responses via clientrpc.
# "http://...": sends request events and waits for the responses to an HTTP(S)
#   server.
# resourcesupstream = pages:/path/to/static/pages
# resourcesupstream = smplestore:/path/to/simple/store
# resourcesupstream = clientrpc
# resourcesupstream = https://example.com

# How to charge for purchases done in simplestore. Either empty (local user
# will manually charge) or "onchain" (generates an on-chain address). Only used
# if the upstream resource processor is of type "simplestore:".
# simplestorepaytype =

# Which account to use when generating an onchain address for simplestore orders.
# If empty, the default account is used.
# simplestoreaccount =

# simplestoreshipcharge is a surcharge (in USD) added to simplestore orders to
# cover shipping and handling.
# simplestoreshipcharge = 0.0
`
)
