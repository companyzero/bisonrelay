package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/companyzero/bisonrelay/brclient/internal/version"
	"github.com/companyzero/bisonrelay/client"
	"github.com/companyzero/bisonrelay/client/clientdb"
	"github.com/companyzero/bisonrelay/client/clientintf"
	"github.com/companyzero/bisonrelay/internal/strescape"
	"github.com/companyzero/bisonrelay/zkidentity"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrlnd/lnrpc"
	"github.com/decred/dcrlnd/lnrpc/walletrpc"
	"github.com/decred/dcrlnd/lnwire"
	"github.com/mitchellh/go-homedir"
	"github.com/skip2/go-qrcode"
	"golang.org/x/exp/slices"
)

const leader = '/'

type tuicmd struct {
	cmd     string
	aliases []string
	usage   string
	descr   string
	long    []string
	sub     []tuicmd

	// usableOffline tracks if the command can be used when we're not
	// connected to a server or don't have a route to pay for server
	// operations.
	usableOffline bool

	handler    func(args []string, as *appState) error
	rawHandler func(rawCmd string, args []string, as *appState) error
	completer  func(prevArgs []string, arg string, as *appState) []string
}

func (cmd *tuicmd) lessThan(other *tuicmd) bool {
	return strings.Compare(cmd.cmd, other.cmd) < 0
}

func (cmd *tuicmd) is(s string) bool {
	if cmd.cmd == s {
		return true
	}
	for _, alias := range cmd.aliases {
		if alias == s {
			return true
		}
	}
	return false
}

// usageError is returned by command handlers to indicate the user typed wrong
// arguments for the given command.
type usageError struct {
	msg string
}

func (err usageError) Error() string {
	return err.msg
}

func (err usageError) Is(target error) bool {
	_, ok := target.(usageError)
	return ok
}

// fileCompleter expands the given arg as an existing filepath and completes
// with the available options.
func fileCompleter(arg string) []string {
	if len(arg) == 0 {
		return nil
	}

	// Handle transforming '~' into home dir.
	if arg[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		return []string{homeDir + arg[1:]}
	}

	// Handle when arg ends in dir separator.
	if arg[len(arg)-1] == '\\' || arg[len(arg)-1] == '/' {
		dirs, _ := os.ReadDir(arg)
		res := make([]string, len(dirs))
		for i, dir := range dirs {
			res[i] = filepath.Join(arg, dir.Name())
		}
		return res
	}

	// Handle when arg is partially a filename.
	dir := filepath.Dir(arg)
	base := filepath.Base(arg)
	var res []string
	entries, _ := os.ReadDir(dir)
	for _, f := range entries {
		if strings.HasPrefix(f.Name(), base) {
			res = append(res, filepath.Join(dir, f.Name()))
		}
	}
	return res
}

// cmdCompleter returns possible completions of the given command.
func cmdCompleter(cmds []tuicmd, arg string, topLevel bool) []string {
	res := make([]string, 0, len(cmds))
	for _, cmd := range cmds {
		if !strings.HasPrefix(cmd.cmd, arg) {
			continue
		}
		s := cmd.cmd + " "
		if topLevel {
			s = string(leader) + s
		}
		res = append(res, s)
	}
	return res
}

// addressbookCompleter returns completions for both nicks and GCs that have a
// prefix.
func addressbookCompleter(arg string, as *appState) []string {
	var res []string
	if nicks := as.c.NicksWithPrefix(arg); len(nicks) > 0 {
		res = append(res, nicks...)
	}
	if aliases := as.c.GCsWithPrefix(arg); len(aliases) > 0 {
		res = append(res, aliases...)
	}
	as.collator.SortStrings(res)
	return res
}

// nickCompleter returns completions for nicks that have a prefix.
func nickCompleter(arg string, as *appState) []string {
	var res []string
	if nicks := as.c.NicksWithPrefix(arg); len(nicks) > 0 {
		res = append(res, nicks...)
	}
	as.collator.SortStrings(res)
	return res
}

// gcCompleter returns completions for GCs that have a prefix.
func gcCompleter(arg string, as *appState) []string {
	var res []string
	if aliases := as.c.GCsWithPrefix(arg); len(aliases) > 0 {
		res = append(res, aliases...)
	}
	as.collator.SortStrings(res)
	return res
}

// subcmdNeededHandler is used on top-level commands that only work with a
// subcommand.
func subcmdNeededHandler(args []string, _ *appState) error {
	if len(args) == 0 {
		return usageError{msg: "subcommand not specified"}
	}
	return usageError{msg: fmt.Sprintf("invalid subcommand %q", args[0])}
}

// handleWithSubcmd returns the handler of the given list of subcommands with
// the given name or panics.
func handleWithSubcmd(subCmds []tuicmd, subCmdName string) func(args []string, as *appState) error {
	for _, sub := range subCmds {
		if sub.cmd == subCmdName {
			return sub.handler
		}
	}
	panic(fmt.Errorf("subcommand %s not found in list of commands", subCmdName))
}

var listCommands = []tuicmd{
	{
		cmd:           "exchangerate",
		usableOffline: true,
		descr:         "Display the current exchange rates",
		handler: func(args []string, as *appState) error {
			dcrPrice, btcPrice := as.rates.Get()
			as.cwHelpMsg(fmt.Sprintf("DCR: %.2f\tBTC: %.2f\t (USD/coin)", dcrPrice, btcPrice))
			return nil
		},
	}, {
		cmd:           "subscribers",
		usableOffline: true,
		aliases:       []string{"subs"},
		descr:         "List people subscribed to the local client's posts",
		handler: func(args []string, as *appState) error {
			subs, err := as.c.ListPostSubscribers()
			if err != nil {
				return err
			}

			as.cwHelpMsgs(func(pf printf) {
				pf("")
				if len(subs) == 0 {
					pf("No subscribers to our posts")
					return
				}

				pf("Post Subscribers")
				for _, sub := range subs {
					userNick, err := as.c.UserNick(sub)
					if err != nil {
						pf("%s (error: %v)", sub, err)
					} else {
						pf("%s - %q", sub, userNick)
					}
				}
			})
			return nil
		},
	}, {
		cmd:           "subscriptions",
		usableOffline: true,
		aliases:       []string{"mysubs"},
		descr:         "List remote users we are subscribed to",
		handler: func(args []string, as *appState) error {
			subs, err := as.c.ListPostSubscriptions()
			if err != nil {
				return err
			}

			as.cwHelpMsgs(func(pf printf) {
				pf("")
				if len(subs) == 0 {
					pf("No post subscriptions")
					return
				}

				pf("Post Subscriptions")
				for _, sub := range subs {
					userNick, err := as.c.UserNick(sub.To)
					if err != nil {
						pf("%s (error: %v)", sub.To, err)
					} else {
						pf("%s - %q", sub.To, userNick)
					}
				}
			})
			return nil
		},
	}, {
		cmd:           "kx",
		usableOffline: true,
		descr:         "List outstanding KX attempts",
		handler: func(args []string, as *appState) error {
			kxs, err := as.c.ListKXs()
			if err != nil {
				return err
			}

			as.cwHelpMsgs(func(pf printf) {
				pf("")
				pf("Active KX attempts")
				for _, kx := range kxs {
					pf("KX %s", kx.InitialRV)
					pf("Stage: %s   IsReset: %v   Updated: %s",
						kx.Stage, kx.IsForReset,
						kx.Timestamp.Format(ISO8601DateTime))
					pf("My Reset RV: %s", kx.MyResetRV)
					if kx.Stage == clientdb.KXStageStep3IDKX {
						pf("Their Reset RV: %s", kx.TheirResetRV)
						pf("Step3RV: %s", kx.Step3RV)
					}
					if kx.Invitee != nil {
						pf("Invitee: %s (%q)", kx.Invitee.Identity,
							strescape.Nick(kx.Invitee.Nick))
					}
					if kx.MediatorID != nil {
						nick, _ := as.c.UserNick(*kx.MediatorID)
						pf("Mediator: %s (%q)", kx.MediatorID,
							strescape.Nick(nick))
					}
					pf("")
				}
			})
			return nil
		},
	}, {
		cmd:           "kxsearches",
		usableOffline: true,
		descr:         "List IDs of clients we're searching for",
		handler: func(args []string, as *appState) error {
			ids, err := as.c.ListKXSearches()
			if err != nil {
				return err
			}

			as.cwHelpMsgs(func(pf printf) {
				pf("")
				pf("KX Searches in progress")
				for _, id := range ids {
					pf(id.String())
				}
			})
			return nil
		},
	}, {
		cmd:           "mediateids",
		usableOffline: true,
		aliases:       []string{"mis"},
		descr:         "List mediate id requests",
		handler: func(args []string, as *appState) error {
			mis, err := as.c.ListMediateIDs()
			if err != nil {
				return err
			}

			sort.Slice(mis, func(i, j int) bool {
				return mis[i].Date.Before(mis[j].Date)
			})

			as.cwHelpMsgs(func(pf printf) {
				pf("")
				pf("Active Mediate ID requests")
				for _, mi := range mis {
					mediatorNick, _ := as.c.UserNick(mi.Mediator)
					targetNick, _ := as.c.UserNick(mi.Target)
					pf("Date: %s", mi.Date.Format(ISO8601DateTime))
					pf("  Mediator: %s (%q)", mi.Mediator, mediatorNick)
					pf("  Target: %s (%q)", mi.Target, targetNick)
					pf("")
				}
			})
			return nil
		},
	}, {
		cmd:           "sharedfiles",
		usableOffline: true,
		descr:         "List files locally shared in the ftp subsystem",
		handler: func(args []string, as *appState) error {
			files, err := as.c.ListLocalSharedFiles()
			if err != nil {
				return nil
			}
			as.cwHelpMsgs(func(pf printf) {
				pf("")
				pf("Shared files")
				for _, f := range files {
					nonGlobal := ""
					if !f.Global {
						nonGlobal = " [privately shared]"
					}
					pf("%s - %s%s (size:%v cost:%0.8f)", f.SF.FID, f.SF.Filename, nonGlobal, f.Size, float64(f.Cost)/1e8)
					if len(f.Shares) > 0 {
						pf("Shared with")
					}
					for _, id := range f.Shares {
						nick, _ := as.c.UserNick(id)
						pf("  %s - %q", id, nick)
					}
				}
			})

			return nil
		},
	}, {
		cmd:           "downloads",
		usableOffline: true,
		descr:         "List in-progress downloads",
		handler: func(args []string, as *appState) error {
			fds, err := as.c.ListDownloads()
			if err != nil {
				return err
			}
			as.cwHelpMsgs(func(pf printf) {
				pf("")
				pf("Downloads")
				for _, fd := range fds {
					nick, _ := as.c.UserNick(fd.UID)
					if nick == "" {
						nick = fd.UID.String()
					}
					pf("%s - %s", fd.FID, strescape.Nick(nick))
					if fd.Metadata == nil {
						pf("(waiting remote metadata)")
						pf("")
						continue
					}
					downChunks := fd.CountChunks(clientdb.ChunkStateDownloaded)
					totalChunks := len(fd.Metadata.Manifest)
					progress := float64(downChunks) / float64(totalChunks) * 100
					pf("Filename: %q", fd.Metadata.Filename)
					pf("Cost: %s", dcrutil.Amount(fd.Metadata.Cost))
					pf("Progress: %.2f (%d/%d)", progress,
						downChunks, totalChunks)
					pf("")
				}
			})
			return nil
		},
	}, {
		cmd:           "paystats",
		usableOffline: true,
		usage:         "[<nick or user id>]",
		descr:         "List payment stats globally or for a specific user",
		handler: func(args []string, as *appState) error {
			if len(args) == 0 {
				stats, err := as.c.ListPaymentStats()
				if err != nil {
					return err
				}

				ids := client.SortedUserPayStatsIDs(stats)
				as.cwHelpMsgs(func(pf printf) {
					pf("")
					pf("Global Payment Statistics")
					pf("        Sent           Recv (DCR)")
					var totalSent, totalRecv int64
					for _, uid := range ids {
						nick, _ := as.c.UserNick(uid)
						nick = ltjustify(strescape.Nick(nick), 12)
						s := stats[uid]
						totSent := float64(s.TotalSent+s.TotalPayFee) / 1e11
						totRecv := float64(s.TotalReceived) / 1e11
						totalSent += s.TotalSent + s.TotalPayFee
						totalRecv += s.TotalReceived
						pf("%12.8f   %12.8f - %s %s",
							totSent, totRecv, nick,
							as.styles.Load().help.Render(uid.String()))
					}
					pf("%12.8f   %12.8f - Totals",
						float64(totalSent)/1e11,
						float64(totalRecv)/1e11)
				})
				return nil
			}

			ru, err := as.c.UserByNick(args[0])
			if err != nil {
				return err
			}

			uid := ru.ID()
			stats, err := as.c.SummarizeUserPayStats(uid)
			if err != nil {
				return err
			}

			as.cwHelpMsgs(func(pf printf) {
				pf("")
				pf("Payment Stats for user %q (%s)", ru.Nick(), uid)
				for _, s := range stats {
					amount := float64(s.Total) / 1e11
					pf("%+12.8f  %s", amount, s.Prefix)
				}
			})

			return nil
		},
	}, {
		cmd:     "svrrates",
		aliases: []string{"serverrates"},
		descr:   "Show server fee rates",
		handler: func(args []string, as *appState) error {
			pushRate, subRate := as.serverPaymentRates()
			as.cwHelpMsgs(func(pf printf) {
				pf("")
				pf("Server Fee Rates")
				pf("Push Rate: %.8f DCR/kB", float64(pushRate)/1e8)
				pf("Subscribe Rate: %.8f DCR/RV", float64(subRate)/1e11)
			})
			return nil
		},
	}, {
		cmd:           "timestats",
		usableOffline: true,
		descr:         "Show timing stats for outbound messages",
		handler: func(args []string, as *appState) error {
			if as.lnPC != nil {
				stats := as.lnPC.PaymentTimingStats()
				as.cwHelpMsgs(func(pf printf) {
					pf("")
					pf("Payment Timing Stats:")
					for _, v := range stats {
						pf("%5s: <= %5dms: %d", v.Rel, v.Max, v.N)
					}
				})
			}

			stats := as.c.RMQTimingStat()
			as.cwHelpMsgs(func(pf printf) {
				pf("")
				pf("Outbound Message Timing Stats:")
				for _, v := range stats {
					pf("%5s: <= %5dms: %d", v.Rel, v.Max, v.N)
				}
			})

			return nil
		},
	}, {
		cmd:           "userslastmsgtime",
		descr:         "List the timestamp of the last message received for every user",
		usableOffline: true,
		handler: func(args []string, as *appState) error {
			users, err := as.c.ListUsersLastReceivedTime()
			if err != nil {
				return nil
			}
			as.cwHelpMsgs(func(pf printf) {
				pf("")
				pf("Last received message time from users (most recent first)")
				for _, user := range users {
					nick, _ := as.c.UserNick(user.UID)
					pf("%s - %s - %s", user.LastDecrypted.Format(ISO8601DateTime),
						strescape.Nick(nick), user.UID)
				}
			})
			return nil
		},
	}, {
		cmd:           "runningtips",
		descr:         "List the currently running tip user attempts",
		usableOffline: true,
		handler: func(args []string, as *appState) error {
			attempts, err := as.c.ListRunningTipUserAttempts()
			if err != nil {
				return err
			}
			as.cwHelpMsgs(func(pf printf) {
				if len(attempts) == 0 {
					pf("No running tip attempts")
				}
				for _, rta := range attempts {
					nick, _ := as.c.UserNick(rta.UID)
					nick = strescape.Nick(nick)
					pf("%s - tag %d - %s - %s", nick,
						rta.Tag, rta.NextAction,
						rta.NextActionTime.Format(ISO8601DateTimeMs))
				}
			})
			return nil
		},
	},
}

var inviteCommands = []tuicmd{
	{
		cmd:   "accept",
		usage: "<filename> [ignoreFunds]",
		descr: "Accept the invite in the given file",
		completer: func(args []string, arg string, as *appState) []string {
			return fileCompleter(arg)
		},
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "filename must be specified"}
			}
			var ignoreFunds bool
			if len(args) > 1 && args[1] == "ignorefunds" {
				ignoreFunds = true
			}

			filename, err := homedir.Expand(args[0])
			if err != nil {
				return err
			}
			f, err := os.Open(filename)
			if err != nil {
				return err
			}
			defer f.Close()

			pii, err := as.c.ReadInvite(f)
			if err != nil {
				return err
			}

			if pii.Funds != nil && !ignoreFunds {
				as.cwHelpMsgs(func(pf printf) {
					pf("")
					pf("Invitation from peer includes funds")
					pf("Nick: %q", pii.Public.Nick)
					pf("UTXO: %s:%d", pii.Funds.Tx, pii.Funds.Index)
					pf("Type '/add %s ignorefunds' to add the invite anyway",
						args[0])
					pf("or '/redeeminvitefunds %s' to redeem the funds on-chain",
						args[0])
				})
				return nil
			}
			as.cwHelpMsgs(func(pf printf) {
				pf("")
				pf("Adding invitation to peer")
				pf("Nick: %q", pii.Public.Nick)
				pf("Name: %q", pii.Public.Name)
				pf("ID: %s", pii.Public.Identity)
			})
			go func() {
				err := as.c.AcceptInvite(pii)
				if err != nil {
					as.cwHelpMsg("Unable to accept invite: %v", err)
				}
			}()
			return nil
		},
	}, {
		cmd:   "new",
		usage: "<filename> [<gcname>]",
		descr: "Create invitation file with optional GC to send OOB to another user",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "filename must be specified"}
			}

			filename, err := homedir.Expand(args[0])
			if err != nil {
				return err
			}

			var gcID zkidentity.ShortID
			if len(args) > 1 && len(args[1]) > 0 {
				gcName := args[1]
				gcID, err = as.c.GCIDByName(gcName)
				if err != nil {
					return err
				}
				if _, err := as.c.GetGC(gcID); err != nil {
					return err
				}
			}

			go as.writeInvite(filename, gcID, nil)
			return nil
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return fileCompleter(arg)
			}
			if len(args) == 1 {
				return gcCompleter(arg, as)
			}
			return nil
		},
	}, {
		cmd:   "fetch",
		usage: "<key> <filename>",
		descr: "Fetches a prepaid invite from the server",
		handler: func(args []string, as *appState) error {
			// TODO: go online if needed and make usableOffline=true

			if len(args) < 1 {
				return usageError{msg: "key cannot be empty"}
			}
			if len(args) < 2 {
				return usageError{msg: "filename cannot be empty"}
			}

			key, err := clientintf.DecodePaidInviteKey(args[0])
			if err != nil {
				return err
			}

			filename, err := homedir.Expand(args[1])
			if err != nil {
				return err
			}
			f, err := os.Create(filename)
			if err != nil {
				return err
			}

			as.diagMsg("Attempting to fetch invite on server (timeout: 1 minute)")
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				defer f.Close()

				invite, err := as.c.FetchPrepaidInvite(ctx, key, f)
				errStyle := as.styles.Load().err
				switch {
				case errors.Is(err, context.DeadlineExceeded):
					as.diagMsg(errStyle.Render("Timeout waiting for invite on server"))
					as.diagMsg("Maybe invite was already fetched?")

				case err != nil:
					msg := fmt.Sprintf("Unable to fetch invite on server: %v", err)
					as.diagMsg(errStyle.Render(msg))

				default:
					as.diagMsg("Fetched invite stored in RV %s and saved "+
						"in file %s", key.RVPoint(), filename)
					if invite.Funds == nil {
						as.diagMsg("Invite has no funds")
					} else {
						as.diagMsg("Invite has funds on output %s:%d",
							invite.Funds.Tx, invite.Funds.Index)
					}
				}
			}()
			return nil
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 1 {
				return fileCompleter(arg)
			}
			return nil
		},
	}, {
		cmd:   "funded",
		usage: "<filename> <fund amount> [<gcname>]",
		descr: "Create invitation file with funds",
		long: []string{
			"The specified amount of DCR will be sent from the default wallet account to the configured invite funding account and the corresponding private key will be included in the created invitation.",
			"The invite funding account may be specified in the config file and cannot be the default wallet account",
		},
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "filename must be specified"}
			}
			if len(args) < 2 {
				return usageError{msg: "amount must be specified"}
			}
			if as.inviteFundsAccount == "" || as.inviteFundsAccount == "default" {
				as.manyDiagMsgsCb(func(pf printf) {
					pf(as.styles.Load().err.Render("Cannot fund invite when funding account is set to the default wallet account"))
					pf("Create a new account with '/ln newaccount <name>'")
					pf("and set the 'invitefundsaccount = <name>' config option in brclient.conf")
				})
				return nil
			}

			filename, err := homedir.Expand(args[0])
			if err != nil {
				return err
			}
			dcrAmount, err := strconv.ParseFloat(args[1], 64)
			if err != nil {
				return usageError{msg: fmt.Sprintf("amount not a valid DCR amount: %v", err)}
			}
			amount, err := dcrutil.NewAmount(dcrAmount)
			if err != nil {
				return err
			}

			var gcID zkidentity.ShortID
			if len(args) > 2 {
				gcName := args[2]
				gcID, err = as.c.GCIDByName(gcName)
				if err != nil {
					return err
				}
				if _, err := as.c.GetGC(gcID); err != nil {
					return err
				}
			}

			funds, err := as.lnPC.CreateInviteFunds(as.ctx, amount, as.inviteFundsAccount)
			if err != nil {
				return err
			}
			as.cwHelpMsg("%s available for invitee after tx %s confirms",
				amount, funds.Tx)

			go as.writeInvite(filename, gcID, funds)
			return nil
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return fileCompleter(arg)
			}
			if len(args) == 1 {
				return gcCompleter(arg, as)
			}
			return nil
		},
	}, {
		cmd:   "redeem",
		usage: "<filename>",
		descr: "Redeem funds included in an invitation",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "filename must be specified"}
			}
			filename, err := homedir.Expand(args[0])
			if err != nil {
				return err
			}
			f, err := os.Open(filename)
			if err != nil {
				return err
			}
			defer f.Close()

			pii, err := as.c.ReadInvite(f)
			if err != nil {
				return err
			}

			total, tx, err := as.lnPC.RedeemInviteFunds(as.ctx, pii.Funds)
			if err != nil {
				return err
			}
			as.cwHelpMsgs(func(pf printf) {
				pf("")
				pf("Redeemed %s as invite funds in tx %s", total, tx)
				pf("The invite can be accepted by issuing the command")
				pf("  /add %s ignorefunds", args[0])
			})
			return nil
		},
		completer: func(args []string, arg string, as *appState) []string {
			return fileCompleter(arg)
		},
	}, {
		cmd:           "qr",
		usableOffline: true,
		usage:         "<invite key> [<path>]",
		descr:         "View or save the QR code of an invite",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "invite key cannot be empty"}
			}

			key := args[0]
			_, err := clientintf.DecodePaidInviteKey(key)
			if err != nil {
				return fmt.Errorf("invalid invite key: %v", err)
			}

			png, err := qrcode.Encode(key, qrcode.Medium, 256)
			if err != nil {
				return fmt.Errorf("unable to encode QR code: %v", err)
			}

			isView := len(args) == 1
			if isView {
				cmd, err := as.viewRaw(png)
				if err != nil {
					return err
				}

				as.sendMsg(msgRunCmd(cmd))
				return nil
			}

			dir := filepath.Dir(args[1])
			if dir != "" {
				err := os.MkdirAll(dir, 0o0700)
				if err != nil {
					return err
				}
			}
			f, err := os.Create(args[1])
			if err != nil {
				return err
			}
			if _, err := f.Write(png); err != nil {
				return err
			}
			if err = f.Close(); err != nil {
				return err
			}
			as.cwHelpMsg("Saved QR code to %s", args[1])
			return nil
		},
	},
}

var gcCommands = []tuicmd{
	{
		cmd:           "new",
		usableOffline: true,
		usage:         "<gc name>",
		descr:         "Create a new group chat named <gc name>",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "gc name cannot be empty"}
			}
			if _, err := as.c.NewGroupChat(args[0]); err != nil {
				return err
			}
			as.cwHelpMsg("GC %q created", args[0])
			return nil
		},
	}, {
		cmd:   "invite",
		usage: "<gc name> <nick>",
		descr: "invite the user with the given nick to join the given gc",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{"gc name cannot be empty"}
			}
			if len(args) < 2 {
				return usageError{"invitee name cannot be empty"}
			}

			gcname, nick := args[0], args[1]
			gcID, err := as.c.GCIDByName(gcname)
			if err != nil {
				return err
			}
			if _, err := as.c.GetGC(gcID); err != nil {
				return err
			}
			cw := as.findOrNewGCWindow(gcID)
			uid, err := as.c.UIDByNick(nick)
			if err != nil {
				return err
			}

			go as.inviteToGC(cw, args[1], uid)
			return nil
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return gcCompleter(arg, as)
			}
			if len(args) == 1 {
				return nickCompleter(arg, as)
			}
			return nil
		},
	},
	{
		cmd:     "msg",
		aliases: []string{"m"},
		usage:   "<gc name> <message>",
		descr:   "send a message to the given GC",
		rawHandler: func(rawCmd string, args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{"gc name cannot be empty"}
			}
			if len(args) < 2 {
				return usageError{"message cannot be empty"}
			}

			gcname := args[0]
			gcID, err := as.c.GCIDByName(gcname)
			if err != nil {
				return err
			}

			_, msg := popNArgs(rawCmd, 3) // cmd + subcmd + gcname

			if _, err := as.c.GetGC(gcID); err != nil {
				return err
			}
			cw := as.findOrNewGCWindow(gcID)
			go as.pm(cw, msg)
			return nil
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return gcCompleter(arg, as)
			}
			return nil
		},
	},
	{
		cmd:   "join",
		usage: "<gc name>",
		descr: "Join the given GC we were invited to",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{"invitation id cannot be empty"}
			}

			gcName := args[0]

			// Try to find it in the db.
			var invite *clientdb.GCInvite
			invites, err := as.c.ListGCInvitesFor(nil)
			if err != nil {
				return err
			}

			for i := len(invites) - 1; i >= 0; i-- {
				if invites[i].Accepted {
					continue
				}
				strId := strconv.Itoa(int(invites[i].ID))
				if invites[i].Invite.Name == gcName || strId == gcName {
					invite = invites[i]
					break
				}
			}

			if invite == nil {
				return fmt.Errorf("unrecognized gc invite %q", gcName)
			}

			go func() {
				err := as.c.AcceptGroupChatInvite(invite.ID)
				if err != nil {
					as.diagMsg("Unable to join gc %q: %v",
						invite.Invite.Name, err)
				} else {
					as.diagMsg("Accepting invitation to "+
						"join gc %q", invite.Invite.Name)
				}
			}()
			return nil
		},
	}, {
		cmd:           "list",
		usableOffline: true,
		usage:         "[<gc name>]",
		aliases:       []string{"l"},
		descr:         "List the GCs we're a member of or members of a GC",
		handler: func(args []string, as *appState) error {
			if len(args) == 0 {
				gcs, err := as.c.ListGCs()
				if err != nil {
					return err
				}
				var maxNameLen int
				gcNames := make(map[clientintf.ID]string, len(gcs))
				for _, gc := range gcs {
					alias, err := as.c.GetGCAlias(gc.ID)
					if err != nil {
						alias = gc.ID.ShortLogID()
					} else {
						alias = strescape.Nick(alias)
					}
					gcNames[gc.ID] = alias
					nameLen := lipgloss.Width(alias)
					if nameLen > maxNameLen {
						maxNameLen = nameLen
					}
				}
				maxNameLen = clamp(maxNameLen, 5, as.winW-64-30)

				sort.Slice(gcs, func(i, j int) bool {
					ni := gcNames[gcs[i].ID]
					nj := gcNames[gcs[j].ID]
					return as.collator.CompareString(ni, nj) < 0
				})

				as.cwHelpMsgs(func(pf printf) {
					pf("")
					pf("List of GCs:")
					for _, gc := range gcs {
						gcAlias := gcNames[gc.ID]
						pf("%*s - %s - %d members",
							maxNameLen,
							truncEllipsis(gcAlias, maxNameLen),
							gc.ID,
							len(gc.Members))
					}
				})

				return nil
			}

			gcID, err := as.c.GCIDByName(args[0])
			if err != nil {
				return err
			}

			gc, err := as.c.GetGC(gcID)
			if err != nil {
				return err
			}
			gcName, _ := as.c.GetGCAlias(gcID)
			if gcName == "" {
				gcName = args[0]
			}

			gcbl, err := as.c.GetGCBlockList(gcID)
			if err != nil {
				return err
			}

			// Collect and sort members according to display order.
			var maxNickW int
			members := slices.Clone(gc.Members[:])
			myID := as.c.PublicID()
			if idx := slices.Index(members, myID); idx > -1 {
				// Remove local client id from list.
				members = slices.Delete(members, idx, idx+1)
			}
			users := make(map[clientintf.UserID]*client.RemoteUser, len(members))
			for _, uid := range members {
				ru, _ := as.c.UserByID(uid)
				users[uid] = ru
				if ru != nil {
					nick := ru.Nick()
					nicklen := lipgloss.Width(nick)
					if nicklen > maxNickW {
						maxNickW = nicklen
					}
				}
			}
			sort.Slice(members, func(i, j int) bool {
				ui := users[members[i]]
				uj := users[members[j]]
				if ui != nil && uj != nil {
					ni := ui.Nick()
					nj := uj.Nick()
					return as.collator.CompareString(ni, nj) < 0
				}
				if uj == nil && ui == nil {
					return members[i].Less(&members[j])
				}
				if uj == nil && ui != nil {
					return true
				}
				return false
			})

			maxNickW = clamp(maxNickW, 5, as.winW-64-10)

			as.cwHelpMsgs(func(pf printf) {
				pf("")
				pf("GC %q - %s", gcName, gc.ID.String())
				pf("Version: %d, Generation: %d, Timestamp: %s",
					gc.Version, gc.Generation,
					time.Unix(gc.Timestamp, 0).Format(ISO8601DateTime))
				if gc.Members[0] == myID {
					pf("Local client is owner of this GC")
				} else if slices.Contains(gc.ExtraAdmins, myID) {
					pf("Local client is admin of this GC")
				}
				pf("Members (%d + local client)", len(members))
				firstUknown := true
				for _, uid := range members {
					var ignored string
					if uid == gc.Members[0] {
						ignored += " (owner)"
					} else if slices.Contains(gc.ExtraAdmins, uid) {
						ignored += " (admin)"
					}
					if gcbl.IsBlocked(uid) {
						ignored += " (in GC blocklist)"
					}
					ru := users[uid]
					if ru == nil {
						if firstUknown {
							pf("")
							pf("Uknonwn or KX incomplete members")
							firstUknown = false
						}
						pf("%s", uid)
					} else {
						nick := strescape.Nick(ru.Nick())
						if len(nick) > maxNickW {
							nick = nick[:maxNickW]
						}
						if ignored == "" && ru.IsIgnored() {
							ignored = " (ignored)"
						}
						pf("%*s - %s%s", maxNickW, nick, uid, ignored)
					}
				}
			})

			return nil
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return gcCompleter(arg, as)
			}
			return nil
		},
	}, {
		cmd:   "kick",
		usage: "<gc> <nick> [<reason>]",
		descr: "Kick the given user from the specified GC",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "gc name cannot be empty"}
			}
			if len(args) < 2 {
				return usageError{msg: "nick cannot be empty"}
			}
			var reason string
			if len(args) > 2 {
				reason = strings.Join(args[2:], " ")
			}
			gcID, err := as.c.GCIDByName(args[0])
			if err != nil {
				return err
			}
			nick := args[1]

			if _, err := as.c.GetGC(gcID); err != nil {
				return err
			}
			var uid clientintf.UserID
			if len(nick) == 64 {
				// Handle gc kicks with UserIDs instead of
				// relying on UIDbyNick to do that to handle
				// kicking blocked users from the GC.
				err := uid.FromString(nick)
				if err != nil {
					return err
				}
			} else {
				uid, err = as.c.UIDByNick(nick)
				if err != nil {
					return err
				}
			}
			gcWin := as.findOrNewGCWindow(gcID)

			go as.kickFromGC(gcWin, uid, nick, reason)
			return nil
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return gcCompleter(arg, as)
			}
			if len(args) == 1 {
				return nickCompleter(arg, as)
			}
			return nil
		},
	}, {
		cmd:   "part",
		usage: "<gc> [<reason>]",
		descr: "Exit from the specified GC",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "gc name cannot be empty"}
			}
			var reason string
			if len(args) > 1 {
				reason = strings.Join(args[1:], " ")
			}
			gcID, err := as.c.GCIDByName(args[0])
			if err != nil {
				return err
			}
			if _, err := as.c.GetGC(gcID); err != nil {
				return err
			}
			gcWin := as.findOrNewGCWindow(gcID)
			go as.partFromGC(gcWin, reason)
			return nil
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return gcCompleter(arg, as)
			}
			return nil
		},
	}, {
		cmd:   "kill",
		usage: "<gc> [<reason>]",
		descr: "Dissolve the specified GC",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "gc name cannot be empty"}
			}
			var reason string
			if len(args) > 1 {
				reason = strings.Join(args[1:], " ")
			}
			gcID, err := as.c.GCIDByName(args[0])
			if err != nil {
				return err
			}
			if _, err := as.c.GetGC(gcID); err != nil {
				return err
			}
			gcWin := as.findOrNewGCWindow(gcID)
			go as.killGC(gcWin, reason)
			return nil
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return gcCompleter(arg, as)
			}
			return nil
		},
	}, {
		cmd:           "ignore",
		usableOffline: true,
		usage:         "<gc> <user>",
		descr:         "Ignore a user's messages in this specific GC",
		long: []string{
			"This also stops sending messages to the specified user in this GC",
		},
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "gc name cannot be empty"}
			}
			if len(args) < 2 {
				return usageError{msg: "user cannot be empty"}
			}

			gcID, err := as.c.GCIDByName(args[0])
			if err != nil {
				return err
			}
			uid, err := as.c.UIDByNick(args[1])
			if err != nil {
				return err
			}

			gcWin := as.findOrNewGCWindow(gcID)
			if err := as.c.AddToGCBlockList(gcID, uid); err != nil {
				return err
			}
			gcWin.newInternalMsg(fmt.Sprintf("Ignored user %s in GC", args[1]))
			as.repaintIfActive(gcWin)
			return nil
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return gcCompleter(arg, as)
			}
			if len(args) == 1 {
				return nickCompleter(arg, as)
			}
			return nil
		},
	}, {
		cmd:           "unignore",
		usableOffline: true,
		usage:         "<gc> <user>",
		descr:         "Un-ignore a user's messages in this specific GC",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "gc name cannot be empty"}
			}
			if len(args) < 2 {
				return usageError{msg: "user cannot be empty"}
			}

			gcID, err := as.c.GCIDByName(args[0])
			if err != nil {
				return err
			}
			uid, err := as.c.UIDByNick(args[1])
			if err != nil {
				return err
			}

			gcWin := as.findOrNewGCWindow(gcID)
			if err := as.c.RemoveFromGCBlockList(gcID, uid); err != nil {
				return err
			}
			gcWin.newInternalMsg(fmt.Sprintf("Un-ignored user %s in GC", args[1]))
			as.repaintIfActive(gcWin)
			return nil
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return gcCompleter(arg, as)
			}
			if len(args) == 1 {
				return nickCompleter(arg, as)
			}
			return nil
		},
	}, {
		cmd:           "alias",
		usableOffline: true,
		usage:         "<existing gc> <new alias>",
		descr:         "Modify the local alias of a GC",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "existing gc name cannot be empty"}
			}
			if len(args) < 2 {
				return usageError{msg: "new gc alias cannot be empty"}
			}

			gcID, err := as.c.GCIDByName(args[0])
			if err != nil {
				return err
			}
			newAlias := args[1]
			if err := as.c.AliasGC(gcID, newAlias); err != nil {
				return err
			}

			gcWin := as.findOrNewGCWindow(gcID)
			gcWin.newInternalMsg(fmt.Sprintf("Renamed GC to %s", newAlias))
			gcWin.alias = newAlias // FIXME: this is racing.
			as.repaintIfActive(gcWin)
			return nil
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return gcCompleter(arg, as)
			}
			return nil
		},
	}, {
		cmd:   "resendlist",
		usage: "<gc> [<user>]",
		descr: "Resends the GC definition to the specified user or all users",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "GC cannot be empty"}
			}

			gcID, err := as.c.GCIDByName(args[0])
			if err != nil {
				return err
			}

			var uid *clientintf.UserID
			if len(args) > 1 {
				user, err := as.c.UIDByNick(args[1])
				if err != nil {
					return err
				}
				uid = &user
			}

			go func() {
				gcWin := as.findOrNewGCWindow(gcID)
				var msg *chatMsg
				if uid == nil {
					msg = gcWin.newInternalMsg("Resending GC list to all GC members")
				} else {
					nick, _ := as.c.UserNick(*uid)
					msg = gcWin.newInternalMsg(fmt.Sprintf("Resending GC list to %q", nick))
				}
				as.repaintIfActive(gcWin)
				err := as.c.ResendGCList(gcID, uid)
				if err != nil {
					gcWin.newHelpMsg("Unable to resent GC List: %v", err)
				} else {
					gcWin.setMsgSent(msg)
				}
				as.repaintIfActive(gcWin)
			}()

			return nil
		},

		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return gcCompleter(arg, as)
			}
			if len(args) == 1 {
				return nickCompleter(arg, as)
			}
			return nil
		},
	}, {
		cmd:   "upgrade",
		usage: "<gc>",
		descr: "Upgrades the GC to the next available version",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "GC cannot be empty"}
			}
			gcID, err := as.c.GCIDByName(args[0])
			if err != nil {
				return err
			}
			gc, err := as.c.GetGC(gcID)
			if err != nil {
				return err
			}
			newVersion := gc.Version + 1
			err = as.c.UpgradeGC(gcID, newVersion)
			if err != nil {
				return err
			}

			cw := as.findOrNewGCWindow(gc.ID)
			cw.newHelpMsg("Upgraded GC to version %d", newVersion)
			as.repaintIfActive(cw)
			return nil
		},

		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return gcCompleter(arg, as)
			}
			return nil
		},
	}, {
		cmd:   "addadmin",
		usage: "<gc> <new admin>",
		descr: "Add a user as an admin of a GC",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "GC cannot be empty"}
			}
			if len(args) < 2 {
				return usageError{msg: "New admin cannot be empty"}
			}

			gcID, err := as.c.GCIDByName(args[0])
			if err != nil {
				return err
			}

			uid, err := as.c.UIDByNick(args[1])
			if err != nil {
				return err
			}

			return as.modifyGCAdmins(gcID, uid, clientintf.UserID{})
		},

		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return gcCompleter(arg, as)
			}
			if len(args) == 1 {
				return nickCompleter(arg, as)
			}
			return nil
		},
	}, {
		cmd:   "deladmin",
		usage: "<gc> <existing admin>",
		descr: "Removes a user as an admin of a GC",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "GC cannot be empty"}
			}
			if len(args) < 2 {
				return usageError{msg: "New admin cannot be empty"}
			}

			gcID, err := as.c.GCIDByName(args[0])
			if err != nil {
				return err
			}

			uid, err := as.c.UIDByNick(args[1])
			if err != nil {
				return err
			}

			return as.modifyGCAdmins(gcID, clientintf.UserID{}, uid)
		},

		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return gcCompleter(arg, as)
			}
			if len(args) == 1 {
				return nickCompleter(arg, as)
			}
			return nil
		},
	}, {
		cmd:   "modowner",
		usage: "<gc> <new owner>",
		descr: "Change the owner of the given GC",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "GC cannot be empty"}
			}
			if len(args) < 2 {
				return usageError{msg: "New admin cannot be empty"}
			}

			gcID, err := as.c.GCIDByName(args[0])
			if err != nil {
				return err
			}
			gcAlias, err := as.c.GetGCAlias(gcID)
			if err != nil {
				return err
			}

			ru, err := as.c.UserByNick(args[1])
			if err != nil {
				return err
			}

			reason := "Changing owner"
			err = as.c.ModifyGCOwner(gcID, ru.ID(), reason)
			if err != nil {
				return err
			}

			cw := as.findOrNewGCWindow(gcID)
			cw.newHelpMsg("Changed owner of GC %s to %s",
				strescape.Nick(gcAlias), strescape.Nick(ru.Nick()))
			as.repaintIfActive(cw)
			return nil
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return gcCompleter(arg, as)
			}
			if len(args) == 1 {
				return nickCompleter(arg, as)
			}
			return nil
		},
	}, {
		cmd:           "listinvites",
		aliases:       []string{"lsinvites"},
		usableOffline: true,
		descr:         "Show outstanding invites to GCs received by the client",
		long:          []string{"When an invite id is specified, show details of that invite."},
		usage:         "[<invite id>]",
		handler: func(args []string, as *appState) error {
			invites, err := as.c.ListGCInvitesFor(nil)
			if err != nil {
				return err
			}

			var showId uint64
			if len(args) > 0 {
				showId, err = strconv.ParseUint(args[0], 10, 64)
				if err != nil {
					return fmt.Errorf("invalid invite id: %v", err)
				}
			}

			acceptedStr := map[bool]string{
				true:  "✓",
				false: " ",
			}

			var shown bool
			as.cwHelpMsgs(func(pf printf) {
				pf("")
				if len(invites) == 0 {
					pf("No GC invites that can be accepted")
					return
				}

				if showId == 0 {
					pf("Outstanding GC Invites")
					pf("%6s %26.26s %20.20s %s %s", "ID", "GC",
						"Inviter", "Acc", "Expiration")
				}
				for _, inv := range invites {
					if showId != 0 && showId != inv.ID {
						continue
					}

					nick, _ := as.c.UserNick(inv.User)
					expiration := time.Unix(inv.Invite.Expires, 0).Format(ISO8601DateTime)
					if showId != 0 {
						pf("Invitation to GC %s", inv.Invite.ID)
						pf("  Invitation ID: %d", inv.ID)
						pf("        GC Name: %s", strescape.Nick(inv.Invite.Name))
						pf("     Inviter ID: %s", inv.User)
						pf("   Inviter Nick: %s", nick)
						pf("     Expiration: %s", expiration)
						pf("       Accepted: %v", inv.Accepted)
						shown = true
						return
					}

					accepted := acceptedStr[inv.Accepted]
					pf("%d %26.26s %20.20s  %s  %s",
						inv.ID, truncEllipsis(strescape.Nick(inv.Invite.Name), 26),
						truncEllipsis(strescape.Nick(nick), 20),
						accepted,
						expiration)
				}
			})

			if showId != 0 && !shown {
				return fmt.Errorf("Invite with id %d not found", showId)
			}
			return nil
		},
	},
}

var ftCommands = []tuicmd{
	{
		cmd:           "share",
		usableOffline: true,
		usage:         "<filename> <cost> [<nick>]",
		descr:         "Share the given file for distribution",
		long: []string{
			"Imports the passed file into the local FTP repository. The cost is specified in DCR.",
			"If a nick or user ID is specified, the file is shared only to that user.",
			"By default, the passed cost is *added* to the estimated upload cost. Use \"=<amount>\" to directly specify the full cost",
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return fileCompleter(arg)
			}
			if len(args) == 2 {
				return nickCompleter(arg, as)
			}
			return nil
		},
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "filename cannot be empty"}
			}
			if len(args) < 2 {
				return usageError{msg: "cost cannot be empty"}
			}
			if len(args[1]) < 1 {
				return usageError{msg: "cost cannot be the empty string"}
			}

			filename, err := homedir.Expand(args[0])
			if err != nil {
				return err
			}
			stat, err := os.Stat(filename)
			if err != nil {
				return err
			}

			var dcrCost, dcrUploadCost float64
			// Figure out upload cost.
			feeRate, _ := as.serverPaymentRates()
			size := stat.Size()
			uploadCost, err := clientintf.EstimateUploadCost(size, feeRate)
			if err != nil {
				return err
			}
			dcrUploadCost = float64(uploadCost) / 1e11

			if args[1][0] == '=' {
				// Exact cost specified.
				dcrCost, err = strconv.ParseFloat(args[1][1:], 64)
				if err != nil {
					return err
				}
			} else {
				// Upload cost + overcharge
				dcrCost, err = strconv.ParseFloat(args[1], 64)
				if err != nil {
					return err
				}
				dcrCost += dcrUploadCost
			}

			var uid *clientintf.UserID
			with := ""
			if len(args) > 2 {
				id, err := as.c.UIDByNick(args[2])
				if err != nil {
					return err
				}
				uid = &id
				with = fmt.Sprintf(" with %q", args[2])
			}
			atomCost := uint64(dcrCost * 1e8)
			sf, _, err := as.c.ShareFile(filename, uid, atomCost, "")
			as.cwHelpMsg("Shared file %q for %.8f DCR (est. cost %.8f DCR)%s. FID: %s",
				sf.Filename, dcrCost, dcrUploadCost, with,
				sf.FID)
			return err

		},
	}, {
		cmd:           "list",
		usableOffline: true,
		aliases:       []string{"ls", "l"},
		usage:         "<nick> <*|shared|global> [<filename_regex>]",
		descr:         "List files of a remote peer",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "nick cannot be empty"}
			}
			if len(args) < 2 {
				return usageError{msg: "dir cannot be empty"}
			}

			uid, err := as.c.UIDByNick(args[0])
			if err != nil {
				return err
			}

			cw := as.findOrNewChatWindow(uid, args[0])
			var filter string
			if len(args) > 2 {
				filter = args[2]
			}
			go as.listUserContent(cw, args[1], filter)
			return nil
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return nickCompleter(arg, as)
			}
			return nil
		},
	}, {
		cmd:   "get",
		usage: "<nick> [<filename> | <FID>]",
		descr: "Fetch the given file from the remote peer",
		long: []string{
			"The file can be referenced either as a filename (in which case the local client must have had a /ft ls issued first) or a full file ID.",
			"If the file requires payment, the remote peer will send an invoice that will be automatically paid before actually receiving the file's contents",
		},
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "nick cannot be empty"}
			}
			if len(args) < 2 {
				return usageError{msg: "filename cannot be empty"}
			}

			uid, err := as.c.UIDByNick(args[0])
			if err != nil {
				return err
			}

			cw := as.findOrNewChatWindow(uid, args[0])
			go as.getUserContent(cw, args[1])
			return nil
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return nickCompleter(arg, as)
			}
			return nil
		},
	}, {
		cmd:           "estimatecost",
		usableOffline: true,
		aliases:       []string{"estcost"},
		descr:         "Estimate cost of uploading the specified file",
		usage:         "<filepath>",
		completer: func(args []string, arg string, as *appState) []string {
			return fileCompleter(arg)
		},
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "file path cannot be empty"}
			}

			fpath := args[0]
			stat, err := os.Stat(fpath)
			if err != nil {
				return err
			}

			feeRate, _ := as.serverPaymentRates()
			size := stat.Size()
			cost, err := clientintf.EstimateUploadCost(size, feeRate)
			if err != nil {
				return err
			}
			as.cwHelpMsg("Cost to upload file (%d B): %s", size,
				dcrutil.Amount(cost/1e3))

			return nil
		},
	}, {
		cmd:           "unshare",
		usableOffline: true,
		usage:         "<file> [<user>]",
		descr:         "Unshare a file",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "file cannot be empty"}
			}
			var user *clientintf.UserID
			if len(args) > 1 {
				uid, err := as.c.UIDByNick(args[1])
				if err != nil {
					return err
				}
				user = &uid
			}

			files, err := as.c.ListLocalSharedFiles()
			if err != nil {
				return nil
			}
			var fid zkidentity.ShortID
			if err := fid.FromString(args[0]); err != nil {
				// Try to find the named file.
				for _, f := range files {
					if f.SF.Filename == args[0] {
						fid = f.SF.FID
						break
					}
				}
				if fid.IsEmpty() {
					return fmt.Errorf("could not find shared file %q",
						args[0])
				}
			}

			err = as.c.UnshareFile(fid, user)
			if err != nil {
				return err
			}

			as.cwHelpMsg("Unshared file %s", fid)
			return nil
		},
	}, {
		cmd:   "send",
		usage: "<user> <filename>",
		descr: "Send a file to user",
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return nickCompleter(arg, as)
			}
			if len(args) == 1 {
				return fileCompleter(arg)
			}
			return nil
		},
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "user cannot be empty"}
			}
			if len(args) < 2 {
				return usageError{msg: "filename cannot be empty"}
			}

			uid, err := as.c.UIDByNick(args[0])
			if err != nil {
				return err
			}
			nick, err := as.c.UserNick(uid)
			if err != nil {
				nick = args[0]
			}

			filename := args[1]
			err = as.c.SendFile(uid, filename)
			if err != nil {
				return err
			}

			cw := as.findChatWindow(uid)
			msg := fmt.Sprintf("Sending file %q to user %q",
				filepath.Base(filename), strescape.Nick(nick))
			if cw == nil {
				as.cwHelpMsg(msg)
			} else {
				cw.newHelpMsg(msg)
			}

			return nil
		},
	}, {
		cmd:           "canceldownload",
		aliases:       []string{"canceldown"},
		descr:         "Cancel an in-progress download",
		usage:         "<file id prefix>",
		usableOffline: true,
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "file id must be specified"}
			}
			fds, err := as.c.ListDownloads()
			if err != nil {
				return err
			}

			var matches []clientintf.FileID
			for _, fd := range fds {
				if strings.HasPrefix(fd.FID.String(), args[0]) {
					matches = append(matches, fd.FID)
				}
			}

			if len(matches) == 0 {
				return fmt.Errorf("file with id %q not found", args[0])
			}
			if len(matches) > 1 {
				return fmt.Errorf("more than one file with id %q exists", args[0])
			}

			err = as.c.CancelDownload(matches[0])
			if err != nil {
				return err
			}
			as.cwHelpMsg("Canceled download of file %s", matches[0])
			return nil
		},
	},
}

var postCommands = []tuicmd{
	{
		cmd:   "new",
		usage: "[<filename>]",
		descr: "Create a new post",
		long:  []string{"If called without arguments, opens the create post window. Otherwise, it creates the post based on the contents of the file."},
		handler: func(args []string, as *appState) error {
			if len(args) > 0 {
				fname, err := homedir.Expand(args[0])
				if err != nil {
					return err
				}

				data, err := os.ReadFile(fname)
				if err != nil {
					return err
				}

				go as.createPost(string(data), filepath.Dir(fname))
				return nil
			}
			as.sendMsg(showNewPostWindow{})
			return nil
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return fileCompleter(arg)
			}
			return nil
		},
	}, {
		cmd:     "external",
		aliases: []string{"ext", "newext"},
		descr:   "Launch $EDITOR to edit a new post",
		handler: func(args []string, as *appState) error {
			go func() {
				post, err := as.editExternalTextFile(baseExternalNewPostContent)
				if err != nil {
					as.cwHelpMsg("Unable to open external editor: %v", err)
					return
				}

				as.createPost(post, "")
			}()
			return nil
		},
	}, {
		cmd:     "subscribe",
		aliases: []string{"sub"},
		usage:   "<nick>",
		descr:   "Subscribe to posts by the given nick",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "nick cannot be empty"}
			}
			uid, err := as.c.UIDByNick(args[0])
			if err != nil {
				return err
			}
			go as.subscribeToPosts(uid)
			return nil
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return nickCompleter(arg, as)
			}
			return nil
		},
	}, {
		cmd:     "allsubscribe",
		aliases: []string{"allsub"},
		descr:   "Subscribe to every remote user's posts",
		handler: func(args []string, as *appState) error {
			as.diagMsg("Attempting to re-subscribe to every remote user's post feed")
			return as.c.SubscribeToAllRemotePosts(nil)
		},
	}, {
		cmd:     "unsubscribe",
		aliases: []string{"unsub"},
		usage:   "<nick>",
		descr:   "Unsubscribe to posts by the given nick",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "nick cannot be empty"}
			}
			uid, err := as.c.UIDByNick(args[0])
			if err != nil {
				return err
			}
			go as.unsubscribeToPosts(uid)
			return nil
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return nickCompleter(arg, as)
			}
			return nil
		},
	}, {
		cmd:     "list",
		aliases: []string{"ls"},
		usage:   "<nick>",
		descr:   "List the posts made by the specified user",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "nick cannot be empty"}
			}
			uid, err := as.c.UIDByNick(args[0])
			if err != nil {
				return err
			}
			cw := as.findOrNewChatWindow(uid, args[0])
			go func() {
				err := as.c.ListUserPosts(uid)
				if err != nil {
					cw.newInternalMsg(fmt.Sprintf("Unable to list user posts: %v", err))
					as.repaintIfActive(cw)
				}
			}()
			cw.newInternalMsg("Listing user posts")
			as.repaintIfActive(cw)
			return nil
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return nickCompleter(arg, as)
			}
			return nil
		},
	}, {
		cmd:   "get",
		usage: "<nick> <post id>",
		descr: "Fetch post written by a remote user",
		long:  []string{"The local client must already be a subscriber of the remote user's posts to be able to fetch an older post."},
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "nick cannot be empty"}
			}
			if len(args) < 2 {
				return usageError{msg: "post id cannot be empty"}
			}

			uid, err := as.c.UIDByNick(args[0])
			if err != nil {
				return err
			}
			var pid clientintf.PostID
			if err := pid.FromString(args[1]); err != nil {
				return err
			}

			cw := as.findOrNewChatWindow(uid, args[0])
			go as.getUserPost(cw, pid)
			return nil
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return nickCompleter(arg, as)
			}
			return nil
		},
	}, {
		cmd:   "relay",
		usage: "<from user> <post id> <to user>",
		descr: "Relay a post made by the from user to the to user",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "from user cannot be empty"}
			}
			if len(args) < 2 {
				return usageError{msg: "post id cannot be empty"}
			}
			if len(args) < 3 {
				return usageError{msg: "to user cannot be empty"}
			}

			fromUID, err := as.c.UIDByNick(args[0])
			if err != nil {
				return fmt.Errorf("from user: %v", err)
			}
			var pid clientintf.PostID
			if err := pid.FromString(args[1]); err != nil {
				return err
			}
			toUID, err := as.c.UIDByNick(args[2])
			if err != nil {
				return fmt.Errorf("to user: %v", err)
			}

			cw := as.findOrNewChatWindow(toUID, args[2])
			go as.relayPost(fromUID, pid, cw)
			return nil
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 || len(args) == 2 {
				return nickCompleter(arg, as)
			}
			return nil
		},
	},
}

var lnCommands = []tuicmd{
	{
		cmd:           "info",
		usableOffline: true,
		descr:         "Show basic LN info",
		handler: func(args []string, as *appState) error {
			if as.lnRPC == nil {
				return fmt.Errorf("LN client not configured")
			}

			info, err := as.lnRPC.GetInfo(as.ctx,
				&lnrpc.GetInfoRequest{})
			if err != nil {
				return err
			}

			as.cwHelpMsgs(func(pf printf) {
				pf("LN Info")
				pf("Node ID: %s", info.IdentityPubkey)
				pf("Version: %s", info.Version)
				pf("Channels Active: %d, Inactive: %d, Pending: %d",
					info.NumActiveChannels, info.NumInactiveChannels,
					info.NumPendingChannels)
				pf("Synced to chain: %v, to graph: %v",
					info.SyncedToChain, info.SyncedToGraph)
				pf("Block height: %d, hash %s", info.BlockHeight,
					info.BlockHash)
			})
			return nil
		},
	},
	{
		cmd:           "newaddress",
		usableOffline: true,
		usage:         "[<account>]",
		descr:         "Create a new standard P2PKH address from the LN wallet",
		handler: func(args []string, as *appState) error {
			var account string
			if len(args) > 0 {
				account = args[0]
			}
			if as.lnRPC == nil {
				return fmt.Errorf("LN client not configured")
			}
			req := &lnrpc.NewAddressRequest{
				Type:    lnrpc.AddressType_PUBKEY_HASH,
				Account: account,
			}
			na, err := as.lnRPC.NewAddress(as.ctx, req)
			if err != nil {
				return err
			}
			as.cwHelpMsg(fmt.Sprintf("Address: %v", na.Address))
			return nil
		},
	},
	{
		cmd:           "listpeers",
		usableOffline: true,
		aliases:       []string{"lspeers"},
		descr:         "List peers the LN node is connected to",
		handler: func(args []string, as *appState) error {
			if as.lnRPC == nil {
				return fmt.Errorf("LN client not configured")
			}
			lpr, err := as.lnRPC.ListPeers(as.ctx,
				&lnrpc.ListPeersRequest{})
			if err != nil {
				return err
			}
			as.cwHelpMsgs(func(pf printf) {
				pf("Peers: %d", len(lpr.Peers))
				for _, p := range lpr.Peers {
					pf("- %s %s", p.PubKey, p.Address)
					pf("  bytesSent:%v bytesRecv:%v", p.BytesSent, p.BytesRecv)
					pf("  sent:%.8f recv:%.8f", dcrutil.Amount(p.AtomsSent).ToCoin(),
						dcrutil.Amount(p.AtomsRecv).ToCoin())
					pf("  inbound:%v pingTime:%v syncType:%v", p.Inbound,
						time.Duration(p.PingTime), p.SyncType)
				}
			})
			return nil
		},
	},
	{
		cmd:           "connectpeer",
		usableOffline: true,
		usage:         "<pubkey@ip:port>",
		descr:         "Connect to LN peer in the format pubkey@host",
		handler: func(args []string, as *appState) error {
			if as.lnRPC == nil {
				return fmt.Errorf("LN client not configured")
			}
			if len(args) < 1 {
				return usageError{msg: "destination cannot be empty"}
			}
			s := strings.Split(args[0], "@")
			if len(s) != 2 {
				return usageError{msg: "destination must be in the form pubkey@host"}
			}
			cpr := lnrpc.ConnectPeerRequest{
				Addr: &lnrpc.LightningAddress{
					Pubkey: s[0],
					Host:   s[1],
				},
				Perm: false,
			}
			as.cwHelpMsg("Attempting to connect to peer %v", args[0])
			go func() {
				_, err := as.lnRPC.ConnectPeer(as.ctx, &cpr)
				if err != nil {
					as.cwHelpMsg("Unable to connect to "+
						"peer %v: %v", args[0], err)
				} else {
					as.cwHelpMsg("Connected to peer %v", args[0])
				}
			}()
			return nil
		},
	},
	{
		cmd:           "disconnectpeer",
		usableOffline: true,
		descr:         "Disconnect from peer identified by pubkey",
		usage:         "<pubkey>",
		handler: func(args []string, as *appState) error {
			if as.lnRPC == nil {
				return fmt.Errorf("LN client not configured")
			}
			if len(args) < 1 {
				return usageError{msg: "pubkey cannot be empty"}
			}
			dpr := lnrpc.DisconnectPeerRequest{
				PubKey: args[0],
			}
			_, err := as.lnRPC.DisconnectPeer(as.ctx, &dpr)
			if err != nil {
				return err
			}
			as.cwHelpMsg(fmt.Sprintf("Disconnected from peer %v", args[0]))
			return nil
		},
	},
	{
		cmd:           "restoremultiscb",
		usableOffline: true,
		descr:         "Restore from a multipacked SCB",
		usage:         "<filename>",
		completer: func(args []string, arg string, as *appState) []string {
			return fileCompleter(arg)
		},
		handler: func(args []string, as *appState) error {
			if as.lnRPC == nil {
				return fmt.Errorf("LN client not configured")
			}
			if len(args) < 1 {
				return usageError{msg: "filename cannot be empty"}
			}
			packedMulti, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("unable to read multi packed "+
					"backup: %v", err)
			}
			_, err = as.lnRPC.RestoreChannelBackups(as.ctx,
				&lnrpc.RestoreChanBackupRequest{
					Backup: &lnrpc.RestoreChanBackupRequest_MultiChanBackup{
						MultiChanBackup: packedMulti,
					},
				})

			if err != nil {
				return err
			}
			as.diagMsg("Applied SCB file successfully")
			return nil
		},
	},
	{
		cmd:           "openchannel",
		usableOffline: true,
		aliases:       []string{"openchan", "opench"},
		descr:         "Open a channel funded by the local node",
		handler: func(args []string, as *appState) error {
			if as.lnRPC == nil {
				return fmt.Errorf("LN client not configured")
			}
			as.sendMsg(msgLNOpenChannel{})
			return nil
		},
	},
	{
		cmd:           "closechannel",
		usableOffline: true,
		aliases:       []string{"closechan"},
		usage:         "<channel-point> [\"force\"]",
		descr:         "Close a local channel",
		long:          []string{"If the remote counterparty is offline, the channel can be force-closed by specifying \"force\" as the second parameter."},
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "channel point cannot be empty"}
			}
			var force bool
			if len(args) > 1 {
				if args[1] != "force" {
					return usageError{msg: "second argument must " +
						"be 'force' or remain empty"}
				}
				force = true
			}

			var chanPoint *lnrpc.ChannelPoint
			if len(args) < 64 {
				// Try to find by the prefix of the channel point.
				chans, err := as.lnRPC.ListChannels(as.ctx,
					&lnrpc.ListChannelsRequest{})
				if err != nil {
					return err
				}
				for _, c := range chans.Channels {
					if strings.HasPrefix(c.ChannelPoint, args[0]) {
						cp, err := strToChanPoint(c.ChannelPoint)
						if err != nil {
							return err
						} else if chanPoint != nil {
							return fmt.Errorf("channel prefix %q "+
								"matches multiple channels", args[0])
						}
						chanPoint = cp
					}
				}
				if chanPoint == nil {
					return fmt.Errorf("channel with "+
						"ChannelPoint prefix %s not found",
						args[0])
				}
			} else {
				var err error
				chanPoint, err = strToChanPoint(args[0])
				if err != nil {
					return err
				}
			}

			as.cwHelpMsg("Requesting channel %s to be closed",
				chanPointToStr(chanPoint))
			go as.closeChannel(chanPoint, force)
			return nil
		},
	},
	{
		cmd:           "fundwallet",
		usableOffline: true,
		descr:         "Fund wallet",
		handler: func(args []string, as *appState) error {
			if as.lnRPC == nil {
				return fmt.Errorf("LN client not configured")
			}
			as.sendMsg(msgLNFundWallet{})
			return nil
		},
	},
	{
		cmd:           "requestrecv",
		usableOffline: true,
		aliases:       []string{"reqrecv"},
		descr:         "Request receive capacity",
		handler: func(args []string, as *appState) error {
			if as.lnRPC == nil {
				return fmt.Errorf("LN client not configured")
			}
			as.sendMsg(msgLNRequestRecv{})
			return nil
		},
	},
	{
		cmd:           "pendingchannels",
		usableOffline: true,
		aliases:       []string{"pendingchans"},
		descr:         "List pending channels",
		handler: func(args []string, as *appState) error {
			if as.lnRPC == nil {
				return fmt.Errorf("LN client not configured")
			}
			chans, err := as.lnRPC.PendingChannels(as.ctx,
				&lnrpc.PendingChannelsRequest{})
			if err != nil {
				return err
			}
			as.cwHelpMsgs(func(pf printf) {
				pf("Pending Open LN Channels: %d", len(chans.PendingOpenChannels))
				for _, c := range chans.PendingOpenChannels {
					remoteNodePub := c.Channel.RemoteNodePub
					capacity := dcrutil.Amount(c.Channel.Capacity).ToCoin()
					localBal := dcrutil.Amount(c.Channel.LocalBalance).ToCoin()
					remoteBal := dcrutil.Amount(c.Channel.RemoteBalance).ToCoin()
					channelPoint := c.Channel.ChannelPoint

					height := c.ConfirmationHeight
					commitFee := dcrutil.Amount(c.CommitFee).ToCoin()
					commitSize := c.CommitSize
					feePerKb := dcrutil.Amount(c.FeePerKb)
					pf("- %s", remoteNodePub)
					pf("  %s cap:%.8f localBal:%.8f remoteBal:%.8f", channelPoint,
						capacity, localBal, remoteBal)
					pf("  confirmHeight:%v commitFee:%.8f commitSize:%v feePerKb:%.8f",
						height, commitFee, commitSize, feePerKb.ToCoin())
				}

				pf("Pending Force-Closed Channels: %d", len(chans.PendingForceClosingChannels))
				for _, c := range chans.PendingForceClosingChannels {
					remoteNodePub := c.Channel.RemoteNodePub
					capacity := dcrutil.Amount(c.Channel.Capacity).ToCoin()
					localBal := dcrutil.Amount(c.Channel.LocalBalance).ToCoin()
					remoteBal := dcrutil.Amount(c.Channel.RemoteBalance).ToCoin()
					channelPoint := c.Channel.ChannelPoint

					closingTx := c.ClosingTxid
					limboBal := dcrutil.Amount(c.LimboBalance).ToCoin()
					matHeight := c.MaturityHeight
					recoveredBal := dcrutil.Amount(c.RecoveredBalance).ToCoin()
					nbHTLCs := len(c.PendingHtlcs)

					pf("- %s", remoteNodePub)
					pf("  %s cap:%.8f localBal:%.8f remoteBal:%.8f", channelPoint,
						capacity, localBal, remoteBal)
					pf("  limboBalance:%.8f  recoveredBalance:%.8f  htlcs:%d",
						limboBal, recoveredBal, nbHTLCs)
					pf("  closingTx:%s maturityHeight:%d", closingTx,
						matHeight)
					for i, htlc := range c.PendingHtlcs {
						amount := dcrutil.Amount(htlc.Amount).ToCoin()
						pf("    HTLC %d: amount:%.8f  incoming:%v  maturityHeight:%d", i, amount, htlc.Incoming, htlc.MaturityHeight)
					}
				}

				pf("Waiting Close Confirmation: %d", len(chans.WaitingCloseChannels))
				for _, c := range chans.WaitingCloseChannels {
					remoteNodePub := c.Channel.RemoteNodePub
					capacity := dcrutil.Amount(c.Channel.Capacity).ToCoin()
					localBal := dcrutil.Amount(c.Channel.LocalBalance).ToCoin()
					remoteBal := dcrutil.Amount(c.Channel.RemoteBalance).ToCoin()
					channelPoint := c.Channel.ChannelPoint
					limbo := dcrutil.Amount(c.LimboBalance).ToCoin()

					pf("- %s", remoteNodePub)
					pf("  %s cap:%.8f localBal:%.8f remoteBal:%.8f limbo:%.8f",
						channelPoint, capacity, localBal, remoteBal, limbo)
				}

			})

			return nil
		},
	},
	{
		cmd:           "chbalance",
		usableOffline: true,
		aliases:       []string{"channelbalance", "chbal"},
		descr:         "Show current channel balances",
		handler: func(args []string, as *appState) error {
			if as.lnRPC == nil {
				return fmt.Errorf("LN client not configured")
			}

			bal, err := as.lnRPC.ChannelBalance(as.ctx,
				&lnrpc.ChannelBalanceRequest{})
			if err != nil {
				return err
			}

			var localBalance, remoteBalance uint64
			if bal.LocalBalance != nil {
				localBalance = bal.LocalBalance.Atoms
			}
			if bal.RemoteBalance != nil {
				remoteBalance = bal.RemoteBalance.Atoms
			}
			msg := fmt.Sprintf("Local Balance: %.8f, Remote Balance: %.8f\n"+
				"Max Inbound: %.8f, Max Outbound: %.8f",
				dcrutil.Amount(localBalance).ToCoin(),
				dcrutil.Amount(remoteBalance).ToCoin(),
				dcrutil.Amount(bal.MaxInboundAmount).ToCoin(),
				dcrutil.Amount(bal.MaxOutboundAmount).ToCoin())

			as.cwHelpMsg(msg)
			return nil
		},
	},
	{
		cmd:           "wbalance",
		usableOffline: true,
		aliases:       []string{"walletbalance", "wbal"},
		descr:         "Show current wallet balance",
		handler: func(args []string, as *appState) error {
			if as.lnRPC == nil {
				return fmt.Errorf("LN client not configured")
			}

			bal, err := as.lnRPC.WalletBalance(as.ctx,
				&lnrpc.WalletBalanceRequest{})
			if err != nil {
				return err
			}

			msg := fmt.Sprintf("Wallet Balance: %.8f, Confirmed: %.8f, "+
				"Unconfirmed: %.8f", dcrutil.Amount(bal.TotalBalance).ToCoin(),
				dcrutil.Amount(bal.ConfirmedBalance).ToCoin(),
				dcrutil.Amount(bal.UnconfirmedBalance).ToCoin())
			as.cwHelpMsg(msg)
			return nil
		},
	},
	{
		cmd:           "channels",
		usage:         "[<debug>]",
		usableOffline: true,
		descr:         "Show list of active channels",
		long:          []string{"If 'debug' is specified, then additional info for the channels is presented"},
		handler: func(args []string, as *appState) error {
			if as.lnRPC == nil {
				return fmt.Errorf("LN client not configured")
			}
			debug := len(args) > 0 && args[0] == "debug"

			chans, err := as.lnRPC.ListChannels(as.ctx,
				&lnrpc.ListChannelsRequest{})
			if err != nil {
				return err
			}

			// Fetch list of aliases.
			nodeAlias := make(map[string]string)
			for _, c := range chans.Channels {
				if _, ok := nodeAlias[c.RemotePubkey]; ok {
					continue
				}
				nodeInfo, _ := as.lnRPC.GetNodeInfo(as.ctx, &lnrpc.NodeInfoRequest{
					PubKey: c.RemotePubkey,
				})
				var alias string
				if nodeInfo != nil {
					alias = nodeInfo.Node.Alias
					if len(alias) > 32 {
						alias = alias[:32]
					}
				} else if len(c.RemotePubkey) > 32 {
					alias = c.RemotePubkey[:32]
				} else {
					nodeAlias[c.RemotePubkey] = c.RemotePubkey
				}
				nodeAlias[c.RemotePubkey] = strescape.Nick(alias)
			}

			as.cwHelpMsgs(func(pf printf) {
				pf("LN Channels: %d", len(chans.Channels))
				if !debug {
					pf("      chan       send                    recv node")
				}
				for _, c := range chans.Channels {
					active := "✓"
					if !c.Active {
						active = "✗"
					}
					shortCP := c.ChannelPoint[:6]
					local := fmt.Sprintf("%.8f", float64(c.LocalBalance)/1e8)
					remote := fmt.Sprintf("%.8f", float64(c.RemoteBalance)/1e8)
					balDisplay := channelBalanceDisplay(c.LocalBalance, c.RemoteBalance)
					pf("  %s %s %s %s %s %s", active, shortCP, local, balDisplay, remote,
						nodeAlias[c.RemotePubkey])

					if debug {
						sid := lnwire.NewShortChanIDFromInt(c.ChanId)
						pf("  cp:%s sid:%s cap:%.8f",
							c.ChannelPoint,
							sid,
							dcrutil.Amount(c.Capacity).ToCoin())
						pf("  pub:%s   localBal:%.8f remoteBal:%.8f",
							c.RemotePubkey,
							dcrutil.Amount(c.LocalBalance).ToCoin(),
							dcrutil.Amount(c.RemoteBalance).ToCoin())
						pf("  unsettled:%.8f updts:%d htlcs:%d",
							dcrutil.Amount(c.UnsettledBalance).ToCoin(),
							c.NumUpdates,
							len(c.PendingHtlcs))
						pf("")
					}
				}
			})
			return nil
		},
	},
	{
		cmd:           "closedchannels",
		aliases:       []string{"closedchans"},
		usableOffline: true,
		descr:         "Show list of closed channels",
		handler: func(args []string, as *appState) error {
			if as.lnRPC == nil {
				return fmt.Errorf("LN client not configured")
			}

			chans, err := as.lnRPC.ClosedChannels(as.ctx,
				&lnrpc.ClosedChannelsRequest{})
			if err != nil {
				return err
			}

			as.cwHelpMsgs(func(pf printf) {
				pf("Closed LN Channels: %d", len(chans.Channels))
				for _, c := range chans.Channels {
					sid := lnwire.NewShortChanIDFromInt(c.ChanId)
					pf("  cp:%s sid:%s cap:%.8f typ: %s",
						c.ChannelPoint,
						sid,
						dcrutil.Amount(c.Capacity).ToCoin(),
						c.CloseType)
					pf("    %s   settled:%.8f timelocked:%.8f",
						c.RemotePubkey,
						dcrutil.Amount(c.SettledBalance).ToCoin(),
						dcrutil.Amount(c.TimeLockedBalance).ToCoin())
					pf("    tx: %s   height: %d",
						c.ClosingTxHash, c.CloseHeight)
				}
			})
			return nil
		},
	},

	{
		cmd:           "svrnode",
		usableOffline: true,
		aliases:       []string{"servernode"},
		descr:         "Show the server LN node info",
		long:          []string{"This also queries the local node for the ability to make payments to the remote server"},
		handler: func(args []string, as *appState) error {
			svrNode := as.c.ServerLNNode()
			if svrNode == "" {
				return fmt.Errorf("client does not have ID of server node")
			}

			return as.queryLNNodeInfo(svrNode, 1)
		},
	}, {
		cmd:           "invoice",
		usableOffline: true,
		usage:         "[amount in DCR] [memo]",
		aliases:       []string{"addinvoice"},
		descr:         "Create an LN invoice",
		rawHandler: func(rawCmd string, args []string, as *appState) error {
			if as.lnRPC == nil {
				return fmt.Errorf("LN client not configured")
			}

			var amount float64
			if len(args) > 0 {
				var err error
				amount, err = strconv.ParseFloat(args[0], 64)
				if err != nil {
					return usageError{msg: fmt.Sprintf("amount is not a number: %v", err)}
				}
			}
			_, memo := popNArgs(rawCmd, 3)

			inv := lnrpc.Invoice{
				Memo:  memo,
				Value: int64(amount * 1e8),
			}
			res, err := as.lnRPC.AddInvoice(as.ctx, &inv)
			if err != nil {
				return err
			}

			as.cwHelpMsg("Create invoice %x", res.RHash)

			// This is needed because wordwrap.String() doesn't break
			// words, only sentences.
			payreq := res.PaymentRequest
			var msg string
			for len(payreq) > 0 {
				if len(payreq) < as.winW {
					msg += payreq
					payreq = ""
				} else {
					msg += payreq[:as.winW-1]
					payreq = payreq[as.winW-1:]
					msg += "\n"
				}
			}
			as.cwHelpMsg(msg)
			return nil
		},
	}, {
		cmd:           "decodeinvoice",
		usableOffline: true,
		usage:         "[invoice]",
		aliases:       []string{"decinvoice", "decodeinv"},
		descr:         "Decode an LN invoice",
		handler: func(args []string, as *appState) error {
			if as.lnRPC == nil {
				return fmt.Errorf("LN client not configured")
			}
			if len(args) < 1 {
				return usageError{msg: "invoice cannot be empty"}
			}
			payreq := strings.TrimPrefix(args[0], "lnpay://")

			req := &lnrpc.PayReqString{PayReq: payreq}
			invoice, err := as.lnRPC.DecodePayReq(as.ctx, req)
			if err != nil {
				return err
			}

			as.cwHelpMsgs(func(pf printf) {
				pf("")
				pf("Decoded Invoice")
				if invoice.NumMAtoms < 1000 {
					pf("Amount: %d MAtoms", invoice.NumMAtoms)
				} else {
					pf("Amount: %s", dcrutil.Amount(invoice.NumAtoms))
				}
				pf("Destination: %s", invoice.Destination)
				expiryTime := time.Unix(invoice.Timestamp, 0).Add(time.Duration(invoice.Expiry) * time.Second)
				pf("Expiry: %s", expiryTime.Format(ISO8601DateTime))
				pf("CLTV expiry: %d blocks", invoice.CltvExpiry)
				pf("Description: %s", strescape.Content(invoice.Description))
			})
			return nil
		},
	}, {
		cmd:           "payinvoice",
		usableOffline: true,
		usage:         "[invoice]",
		aliases:       []string{"sendpayment", "pay"},
		descr:         "Pay an LN invoice",
		handler: func(args []string, as *appState) error {
			if as.lnRPC == nil {
				return fmt.Errorf("LN client not configured")
			}
			if len(args) < 1 {
				return usageError{msg: "invoice cannot be empty"}
			}

			payreq := strings.TrimPrefix(args[0], "lnpay://")

			as.cwHelpMsg("Attempting to pay invoice")
			go func() {
				pc, err := as.lnRPC.SendPayment(as.ctx)
				if err != nil {
					as.cwHelpMsg("PC: %v", err)
					return
				}

				req := &lnrpc.SendRequest{
					PaymentRequest: payreq,
				}
				err = pc.Send(req)
				if err != nil {
					as.cwHelpMsg("Unable to start payment: %v", err)
				}
				res, err := pc.Recv()
				if err != nil {
					as.cwHelpMsg("PC receive error: %v", err)
					return
				}
				if res.PaymentError != "" {
					as.cwHelpMsg("Payment error: %s", res.PaymentError)
					return
				}
				as.cwHelpMsg("Payment done!")
			}()

			return nil
		},
	}, {
		cmd:           "queryroute",
		usableOffline: true,
		usage:         "<dest node ID> [amount in atoms]",
		descr:         "Query a route to a destination",
		rawHandler: func(rawCmd string, args []string, as *appState) error {
			if as.lnRPC == nil {
				return fmt.Errorf("LN client not configured")
			}
			if len(args) < 1 {
				return usageError{msg: "dest node ID cannot be empty"}
			}
			var amount uint64 = 1
			if len(args) > 1 {
				var err error
				amount, err = strconv.ParseUint(args[1], 10, 64)
				if err != nil {
					return usageError{msg: fmt.Sprintf("amount is not a number: %v", err)}
				}
			}

			return as.queryLNNodeInfo(args[0], amount)
		},
	}, {
		cmd:           "sendonchain",
		usage:         "<DCR amount> <dest address> [<source account>]",
		descr:         "Send funds from the on-chain wallet",
		usableOffline: true,
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "amount cannot be empty"}
			}
			if len(args) < 2 {
				return usageError{msg: "destination address cannot be empty"}
			}
			var account string
			if len(args) > 2 {
				account = args[2]
			}
			dcrAmount, err := strconv.ParseFloat(args[0], 64)
			if err != nil {
				return fmt.Errorf("amount is not valid: %v", err)
			}
			if dcrAmount <= 0 {
				return usageError{msg: "cannot send non-positive dcr amount"}
			}
			amount, err := dcrutil.NewAmount(dcrAmount)
			if err != nil {
				return err
			}

			addr := args[1]
			as.cwHelpMsg("Sending %s DCR to %s", amount, addr)
			go func() {
				req := &lnrpc.SendCoinsRequest{
					Addr:    addr,
					Amount:  int64(amount),
					Account: account,
				}
				res, err := as.lnRPC.SendCoins(as.ctx, req)
				if err != nil {
					as.cwHelpMsg("Usable to send coins on-chain: %v", err)
					return
				}
				as.cwHelpMsg("Sent coins through tx %s", res)
			}()
			return nil
		},
	}, {
		cmd:           "debuglevel",
		usage:         "<level>",
		usableOffline: true,
		descr:         "Change the debug level of the internal LN wallet",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "debug level cannot be empty"}
			}
			req := &lnrpc.DebugLevelRequest{
				LevelSpec: args[0],
			}
			_, err := as.lnRPC.DebugLevel(as.ctx, req)
			return err
		},
	}, {
		cmd:           "accounts",
		usableOffline: true,
		descr:         "List wallet accounts",
		handler: func(args []string, as *appState) error {
			res, err := as.lnWallet.ListAccounts(as.ctx, &walletrpc.ListAccountsRequest{})
			if err != nil {
				return err
			}

			bal, err := as.lnRPC.WalletBalance(as.ctx, &lnrpc.WalletBalanceRequest{})
			if err != nil {
				return err
			}

			as.cwHelpMsgs(func(pf printf) {
				pf("")
				pf("Wallet accounts (%d)", len(res.Accounts))
				for _, acc := range res.Accounts {
					acctBal := dcrutil.Amount(bal.AccountBalance[acc.Name].ConfirmedBalance + bal.AccountBalance[acc.Name].UnconfirmedBalance)
					pf("%s - %s - keys: %d internal, %d external",
						acc.Name, acctBal, acc.InternalKeyCount,
						acc.ExternalKeyCount)
				}
			})
			return nil
		},
	}, {
		cmd:           "newaccount",
		usage:         "<name>",
		usableOffline: true,
		descr:         "Create a new wallet account",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "account name cannot be empty"}
			}
			name := strings.TrimSpace(args[0])
			if len(name) == 0 {
				return usageError{msg: "account name cannot be empty string"}
			}

			_, err := as.lnWallet.DeriveNextAccount(as.ctx, &walletrpc.DeriveNextAccountRequest{Name: name})
			if err != nil {
				return err
			}
			as.cwHelpMsg("Created account %s", name)
			return nil
		},
	}, {
		cmd:           "rescanwallet",
		usage:         "[<start height>]",
		usableOffline: true,
		descr:         "Rescan for on-chain wallet transactions",
		handler: func(args []string, as *appState) error {
			var beginHeight int32
			if len(args) > 0 {
				h, err := strconv.ParseInt(args[0], 10, 32)
				if err != nil {
					return err
				}
				beginHeight = int32(h)
			}

			go func() {
				req := &walletrpc.RescanWalletRequest{BeginHeight: beginHeight}
				s, err := as.lnWallet.RescanWallet(as.ctx, req)
				if err != nil {
					as.cwHelpMsg("Unable to rescan wallet: %v", err)
					return
				}

				t := time.Now()
				as.cwHelpMsg("Starting rescan at height %d", beginHeight)
				var lastHeight int32
				ntf, err := s.Recv()
				for ; err == nil; ntf, err = s.Recv() {
					if time.Since(t) > 5*time.Second {
						as.cwHelpMsg("Rescanned up to block %d", ntf.ScannedThroughHeight)
						t = time.Now()
					}
					lastHeight = ntf.ScannedThroughHeight
				}
				if err == nil || errors.Is(err, io.EOF) {
					as.cwHelpMsg("Finished rescan at height %d", lastHeight)
				} else {
					as.cwHelpMsg("Error during rescan (last height %d): %v",
						lastHeight, err)
				}
			}()
			return nil
		},
	}, {
		cmd:           "listtxs",
		usableOffline: true,
		aliases:       []string{"listtransactions"},
		descr:         "List on-chain wallet transactions",
		usage:         "[<start height>] [<end height>]",
		long: []string{
			"If start height is not specified, it defaults to -1. If end height is not specified, it defaults to 0.",
			"This makes the listing include unconfirmed transactions and returns most recent transactions first.",
		},
		handler: func(args []string, as *appState) error {
			var startHeight, endHeight int32 = 0, 0
			if len(args) > 0 {
				i, err := strconv.ParseInt(args[0], 10, 32)
				if err != nil {
					return fmt.Errorf("start height parsing error: %v", err)
				}
				startHeight = int32(i)
			}
			if len(args) > 1 {
				i, err := strconv.ParseInt(args[1], 10, 32)
				if err != nil {
					return fmt.Errorf("end height parsing error: %v", err)
				}
				endHeight = int32(i)
			}

			// TODO: What if GetTransactions it too large?
			as.cwHelpMsg("Fetching list of transactions...")
			go func() {
				req := &lnrpc.GetTransactionsRequest{StartHeight: startHeight, EndHeight: endHeight}
				txs, err := as.lnRPC.GetTransactions(as.ctx, req)
				if err != nil {
					errMsg := fmt.Sprintf("Unable to list transactions: %v", err)
					as.diagMsg(as.styles.Load().err.Render(errMsg))
					return
				}

				as.cwHelpMsgs(func(pf printf) {
					pf("")
					pf("Wallet transactions")
					pf("       Net Amount -  Height - Tx Hash")
					for _, tx := range txs.Transactions {
						value := dcrutil.Amount(tx.Amount)
						pf("%13.8f DCR - %7d - %s",
							value.ToCoin(), tx.BlockHeight, tx.TxHash)
					}
				})
			}()
			return nil
		},
	},
}

var pagesCommands = []tuicmd{
	{
		cmd:   "view",
		descr: "View user page",
		usage: "<nick> [<path/to/page>]",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "nick cannot be empty"}
			}
			uid, err := as.c.UIDByNick(args[0])
			if err != nil {
				return err
			}

			nextSess, err := as.c.NewPagesSession()
			if err != nil {
				return err
			}

			pagePath := "index.md"
			if len(args) > 1 {
				pagePath = strings.TrimSpace(args[1])
			}
			return as.fetchPage(uid, pagePath, nextSess, 0, nil, "")
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return nickCompleter(arg, as)
			}
			return nil
		},
	}, {
		cmd:           "local",
		descr:         "View local user page",
		usage:         "[<path/to/page>]",
		usableOffline: true,
		handler: func(args []string, as *appState) error {
			pagePath := "index.md"
			if len(args) > 1 {
				pagePath = strings.TrimSpace(args[1])
			}

			// Always use the same session ID for convenience when
			// fetching a local page.
			nextSess := clientintf.PagesSessionID(0)
			return as.fetchPage(as.c.PublicID(), pagePath, nextSess, 0, nil, "")
		},
	},
}

var filterCommands = []tuicmd{
	{
		cmd:           "list",
		usableOffline: true,
		aliases:       []string{"ls"},
		descr:         "Lists content-based filters",
		handler: func(args []string, as *appState) error {
			filters := as.c.ListContentFilters()
			if len(filters) == 0 {
				as.cwHelpMsg("Client has no content-based filters")
				return nil
			}

			as.cwHelpMsgs(func(pf printf) {
				pf("")
				pf("Content filters (%d total)", len(filters))
				for _, cf := range filters {
					var s string
					if !cf.SkipPMs {
						s += "PMs, "
					}
					if !cf.SkipGCMs {
						s += "GCMs"
						if cf.GC != nil && !cf.GC.IsEmpty() {
							gc, _ := as.c.GetGCAlias(*cf.GC)
							s += fmt.Sprintf(" gc=%s", strescape.Nick(gc))
						}
						s += ", "

					}
					if !cf.SkipPosts {
						s += "posts, "
					}
					if !cf.SkipPostComments {
						s += "post comments, "
					}
					if cf.UID != nil {
						nick, _ := as.c.UserNick(*cf.UID)
						s += fmt.Sprintf("user=%s, ", strescape.Nick(nick))
					}

					s += fmt.Sprintf("regexp=\"%s\"", cf.Regexp)
					pf("%08d - %s", cf.ID, s)
				}
			})

			return nil
		},
	}, {
		cmd:           "add",
		usableOffline: true,
		descr:         "Add a simple, case-insensitive content filter",
		usage:         "[filter]",
		long: []string{
			"Adding a filter through this command filters content from ",
			"all users on all contexts. Use the 'addrule' command for ",
			"specifying more complex rules.",
		},
		rawHandler: func(rawCmd string, args []string, as *appState) error {
			_, expr := popNArgs(rawCmd, 2) // cmd+subcmd
			if len(expr) == 0 {
				return usageError{"filter expression cannot be empty"}
			}

			// Add the case-insensitive flag.
			expr = "(?i)" + regexp.QuoteMeta(expr)

			cf := &clientdb.ContentFilter{Regexp: expr}
			err := as.c.StoreContentFilter(cf)
			if err != nil {
				return err
			}
			as.cwHelpMsg("Added content filter rule %d", cf.ID)
			return nil
		},
	}, {
		cmd:           "addrule",
		usableOffline: true,
		descr:         "Add a content-based filter",
		usage:         "[user=<user>] [gc=<gc> | noGC] [noPost] [noPC] [noPM] [--] [regexp]",
		long: []string{
			"Content-based filters drop received messages before they are ",
			"presented to the user.",
			"",
			"If an incoming message matches any of the existing filters, ",
			"the message is dropped.",
			"",
			"By default, filters apply to all messages, received from any ",
			"user and on all contexts (PMs, GCMs, posts and post comments). ",
			"Some attributes allow refining when the filter applies: ",
			"",
			"  - user=<user>: only test filter if the message is from a specific user",
			"  - gc=<gc>: only test filter if GCM is in a specific GC",
			"  - noGC: do not apply filter for messages in GCs",
			"  - noPost: do not apply filter for post conent",
			"  - noPC: do not apply filter for post comments",
			"  - noPM: do not apply filter for PMs",
			"",
			"Everything after the last valid option or after a literal '--'",
			"is considered part of the regexp.",
			"",
			"The regexp follows Go's regexp package rules. As a reminder, ",
			"it is case-sensitive by default, unless it is started with ",
			"the '(?i)' flag.",
			"",
			"Examples of filters:",
			"",
			"- Filter all posts and post comments from user foo that contain ",
			"the string 'barbaz':",
			"",
			"    /filter addrule user=foo nogc nopm -- barbaz",
			"",
			"- Filter all messages in GC 'testgc' that contain the string 'barbaz':",
			"",
			"    /filter addrule gc=testgc nopm nopost nopc -- barbaz",
			"",
			"- Filter any message that begins with the case insensitive string 'barbaz':",
			"",
			"    /filter addrule (?i)^barbaz",
			"",
			"Use the /filter test* commands to test if the setup filters work as ",
			"needed.",
		},
		rawHandler: func(rawCmd string, args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "filter cannot be empty"}
			}

			var cf clientdb.ContentFilter
			nargs := 2 // cmd+subcmd
		loopArgs:
			for _, arg := range args {
				switch {
				case strings.HasPrefix(arg, "user="):
					nick := arg[5:]
					uid, err := as.c.UIDByNick(nick)
					if err != nil {
						return err
					}
					cf.UID = &uid
					nargs += 1

				case strings.HasPrefix(arg, "gc="):
					name := arg[3:]
					gc, err := as.c.GCIDByName(name)
					if err != nil {
						return err
					}
					cf.GC = &gc
					nargs += 1

				case strings.EqualFold(arg, "noPost"):
					cf.SkipPosts = true
					nargs += 1

				case strings.EqualFold(arg, "noPC"):
					cf.SkipPostComments = true
					nargs += 1

				case strings.EqualFold(arg, "noGC"):
					cf.SkipGCMs = true
					nargs += 1

				case strings.EqualFold(arg, "noPM"):
					cf.SkipPMs = true
					nargs += 1

				case arg == "--":
					nargs += 1
					break loopArgs

				default:
					break loopArgs
				}
			}

			_, cf.Regexp = popNArgs(rawCmd, nargs)
			if cf.Regexp == "" {
				return usageError{msg: "regexp cannot be empty"}
			}

			err := as.c.StoreContentFilter(&cf)
			if err != nil {
				return err
			}

			as.cwHelpMsg("Added content filter rule %d", cf.ID)
			return nil
		},
	}, {
		cmd:     "del",
		aliases: []string{"delete", "remove", "rem"},
		descr:   "Remove a filter rule",
		usage:   "[id]",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "rule number cannot be empty"}
			}

			id, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return err
			}

			if err := as.c.RemoveContentFilter(id); err != nil {
				return fmt.Errorf("unable to remove rule %d: %v", id, err)
			}

			as.cwHelpMsg("Removed content filter rule %d", id)
			return nil
		},
	}, {
		cmd:           "testpm",
		usableOffline: true,
		descr:         "Test if a PM would be filtered",
		usage:         "<user> <pm>",
		rawHandler: func(rawCmd string, args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "user cannot be empty"}
			}

			uid, err := as.c.UIDByNick(args[0])
			if err != nil {
				return err
			}

			_, pm := popNArgs(rawCmd, 3) // cmd+subcmd+user
			filter, id := as.c.FilterPM(uid, pm)
			as.cwHelpMsgs(func(pf printf) {
				pf("")
				pf("Test message: %q", pm)
				if !filter {
					pf("Message would NOT be filtered")
				} else {
					pf("Message would be filtered by rule %d", id)
				}
			})
			return nil
		},
	}, {
		cmd:           "testgcm",
		usableOffline: true,
		descr:         "Test if a GC message would be filtered",
		usage:         "<user> <gc> <pm>",
		rawHandler: func(rawCmd string, args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "user cannot be empty"}
			}
			if len(args) < 2 {
				return usageError{msg: "gc cannot be empty"}
			}

			uid, err := as.c.UIDByNick(args[0])
			if err != nil {
				return err
			}
			gc, err := as.c.GCIDByName(args[1])
			if err != nil {
				return err
			}

			_, gcm := popNArgs(rawCmd, 4) // cmd+subcmd+user+gc
			filter, id := as.c.FilterGCM(uid, gc, gcm)
			as.cwHelpMsgs(func(pf printf) {
				pf("")
				pf("Test message: %q", gcm)
				if !filter {
					pf("Message would NOT be filtered")
				} else {
					pf("Message would be filtered by rule %d", id)
				}
			})
			return nil
		},
	}, {
		cmd:           "testpost",
		usableOffline: true,
		descr:         "Test if a post would be filtered",
		usage:         "<user> <post>",
		rawHandler: func(rawCmd string, args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "user cannot be empty"}
			}

			uid, err := as.c.UIDByNick(args[0])
			if err != nil {
				return err
			}

			_, post := popNArgs(rawCmd, 3) // cmd+subcmd+user
			filter, id := as.c.FilterPost(uid, clientintf.PostID{}, post)
			as.cwHelpMsgs(func(pf printf) {
				pf("")
				pf("Test post: %q", post)
				if !filter {
					pf("Post would NOT be filtered")
				} else {
					pf("Post would be filtered by rule %d", id)
				}
			})
			return nil
		},
	}, {
		cmd:           "testpostcomment",
		usableOffline: true,
		descr:         "Test if a post comment would be filtered",
		usage:         "<user> <post>",
		rawHandler: func(rawCmd string, args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "user cannot be empty"}
			}

			uid, err := as.c.UIDByNick(args[0])
			if err != nil {
				return err
			}

			_, comment := popNArgs(rawCmd, 3) // cmd+subcmd+user
			postFrom, pid := clientintf.UserID{0: 0x01}, clientintf.UserID{0: 0x02}
			filter, id := as.c.FilterPostComment(uid, postFrom, pid, comment)
			as.cwHelpMsgs(func(pf printf) {
				pf("")
				pf("Test comment: %q", comment)
				if !filter {
					pf("Comment would NOT be filtered")
				} else {
					pf("Comment would be filtered by rule %d", id)
				}
			})
			return nil
		},
	},
}

var myAvatarCmds = []tuicmd{
	{
		cmd:   "set",
		descr: "Set the user avatar",
		usage: "<filename>",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return fmt.Errorf("filename cannot be empty")
			}

			avatar, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}

			if !strings.HasPrefix(imageMimeType(avatar), "image/") {
				return fmt.Errorf("unsupported file format")
			}

			err = as.c.UpdateLocalAvatar(avatar)
			if err != nil {
				return err
			}

			as.diagMsg("Avatar updated!")
			return nil
		},
		completer: func(args []string, arg string, as *appState) []string {
			return fileCompleter(arg)
		},
	}, {
		cmd:   "clear",
		descr: "Removes the user avatar",
		long:  []string{"An update is sent to remote users to clear the avatar."},
		handler: func(args []string, as *appState) error {
			err := as.c.UpdateLocalAvatar(nil)
			if err != nil {
				return err
			}
			as.diagMsg("Avatar updated")
			return nil
		},
	}, {
		cmd:           "view",
		descr:         "View the user avatar",
		usableOffline: true,
		handler: func(args []string, as *appState) error {
			pubid := as.c.Public()
			if len(pubid.Avatar) == 0 {
				return fmt.Errorf("user does not have avatar")
			}
			cmd, err := as.viewRaw(pubid.Avatar)
			if err != nil {
				return err
			}
			as.sendMsg(msgRunCmd(cmd))
			return nil
		},
	},
}

var commands = []tuicmd{
	{
		cmd:           "backup",
		usableOffline: true,
		descr:         "Create a backup of brclient",
		usage:         "<backup-directory>",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "backup directory cannot be empty"}
			}
			destPath := cleanAndExpandPath(args[0])

			backupFile, err := as.c.Backup(as.ctx, as.rootDir, destPath)
			if err != nil {
				return err
			}
			as.log.Infof("Successfully backed up to %v", backupFile)
			return nil
		},
	}, {
		cmd:           "online",
		usableOffline: true,
		descr:         "Attempt to connect and remain connected to the server",
		handler: func(args []string, as *appState) error {
			as.c.GoOnline()
			return nil
		},
	}, {
		cmd:           "offline",
		usableOffline: true,
		descr:         "Disconnect from server and remain offline",
		handler: func(args []string, as *appState) error {
			as.c.RemainOffline()
			return nil
		},
	}, {
		cmd:           "enablecanpay",
		usableOffline: true,
		descr:         "Force-enable the 'can pay server' flag",
		long: []string{"This allows bypassing the internal test that checks whether the client can reach the server LN node before sending commands.",
			"This is sometimes needed when routing estimation in the underlying LN node is wrong.",
			"Once force-enabled, this remains enabled until the app is closed."},
		handler: func(args []string, as *appState) error {
			as.canPayServerMtx.Lock()
			as.canPayServer = true
			as.canPayServerTestTime = time.Now().Add(time.Hour * 24 * 365)
			as.canPayServerMtx.Unlock()
			as.cwHelpMsg("Force-enabled 'can pay server' flag")
			return nil
		},
	}, {
		cmd:           "debuglevel",
		usableOffline: true,
		aliases:       []string{"dlvl"},
		usage:         "<level | subsys=level>",
		descr:         "Set the log level of the application subsystems",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "level cannot be empty"}
			}
			err := as.logBknd.setLogLevel(args[0])
			if err != nil {
				return err
			}
			as.log.Infof("Modified log level to: %q", args[0])
			return nil
		},
	}, {
		cmd:           "ignore",
		usableOffline: true,
		usage:         "<nick>",
		descr:         "Ignore user messages",
		long:          []string{"This maintains internal communication with the user but avoids surfacing private and group messages from them."},
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "user cannot be empty"}
			}
			uid, err := as.c.UIDByNick(args[0])
			if err != nil {
				return err
			}
			return as.c.Ignore(uid, true)
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return nickCompleter(arg, as)
			}
			return nil
		},
	}, {
		cmd:           "unignore",
		usableOffline: true,
		usage:         "<nick>",
		descr:         "Unignore user",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "user cannot be empty"}
			}
			uid, err := as.c.UIDByNick(args[0])
			if err != nil {
				return err
			}
			return as.c.Ignore(uid, false)
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return nickCompleter(arg, as)
			}
			return nil
		},
	}, {
		cmd:   "block",
		usage: "<nick>",
		descr: "Remove and block further contact with the user",
		long: []string{
			"The remote user is asked not to send any more messages to the local client.",
		},
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "nick cannot be empty"}
			}

			uid, err := as.c.UIDByNick(args[0])
			if err != nil {
				return err
			}
			cw := as.findOrNewChatWindow(uid, args[0])
			go as.block(cw)
			return nil
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return nickCompleter(arg, as)
			}
			return nil
		},
	}, {
		cmd:     "list",
		aliases: []string{"l", "ls"},
		usage:   "[subcmd]",
		descr:   "List various things",
		sub:     listCommands,
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return cmdCompleter(listCommands, arg, false)
			}
			return nil
		},
		handler: subcmdNeededHandler,
	}, {
		cmd:           "query",
		usableOffline: true,
		aliases:       []string{"q"},
		usage:         "<nick>",
		descr:         "Open a chat window with the provided nick",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "nick cannot be empty"}
			}
			return as.openChatWindow(args[0])
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return addressbookCompleter(arg, as)
			}
			return nil
		},
	}, {
		cmd:           "addressbook",
		usage:         "[<user>] [viewavatar]",
		usableOffline: true,
		aliases:       []string{"ab"},
		long: []string{
			"Without arguments, shows the entire addressbook (list of known users).",
			"",
			"If passing a nick on the first argument, it shows detailed addressbook information for that user.",
			"",
			"If the user has an avatar, passing 'viewavatar' will open the external viewer for that avatar image type.",
		},
		descr: "Show the address book of known remote users (or a user profile)",
		handler: func(args []string, as *appState) error {
			var viewAvatar bool
			if len(args) > 1 {
				switch {
				case args[1] == "viewavatar":
					viewAvatar = true
				default:
					return fmt.Errorf("unknown argument %q", args[1])
				}
			}

			if len(args) > 0 {
				ru, err := as.c.UserByNick(args[0])
				if err != nil {
					return err
				}

				ab, err := as.c.AddressBookEntry(ru.ID())
				if err != nil {
					return err
				}

				if viewAvatar {
					if len(ab.ID.Avatar) == 0 {
						return fmt.Errorf("user does not have an avatar")
					}
					cmd, err := as.viewRaw(ab.ID.Avatar)
					if err != nil {
						return err
					}
					as.sendMsg(msgRunCmd(cmd))
					return nil
				}

				as.cwHelpMsgs(func(pf printf) {
					r := ru.RatchetDebugInfo()
					pf("")
					pf("Info for user %s", strescape.Nick(ru.Nick()))
					pf("              UID: %s", ru.ID())
					if ru.Nick() != ab.ID.Nick {
						pf("    Original Nick: %s", strescape.Nick(ab.ID.Nick))
					}
					pf("             Name: %s", strescape.Content(ab.ID.Name))
					pf("          Ignored: %v", ru.IsIgnored())
					pf("    First Created: %s", ab.FirstCreated.Format(ISO8601DateTimeMs))
					pf("Last Completed KX: %s", ab.LastCompletedKX.Format(ISO8601DateTimeMs))
					pf("Handshake Attempt: %s", ab.LastHandshakeAttempt.Format(ISO8601DateTimeMs))
					pf("Last Encrypt Time: %s", r.LastEncTime.Format(ISO8601DateTimeMs))
					pf("Last Decrypt Time: %s", r.LastDecTime.Format(ISO8601DateTimeMs))
					pf("          Send RV: %s (%s...)",
						r.SendRVPlain, r.SendRV.ShortLogID())
					pf("          Recv RV: %s (%s...)",
						r.RecvRVPlain, r.RecvRV.ShortLogID())
					pf("         Drain RV: %s (%s...)",
						r.DrainRVPlain, r.DrainRV.ShortLogID())
					pf("      My Reset RV: %s", r.MyResetRV)
					pf("   Their Reset RV: %s", r.TheirResetRV)
					pf("       Saved Keys: %d", r.NbSavedKeys)
					pf("     Will Ratchet: %v", r.WillRatchet)
					if len(ab.ID.Avatar) > 0 {
						pf("View user's avatar with the following command:")
						pf("  /ab %s viewavatar", args[0])
					}
				})
				return nil
			}

			ab := as.c.AllRemoteUsers()
			var maxNickLen int
			for i := range ab {
				l := lipgloss.Width(ab[i].Nick())
				if l > maxNickLen {
					maxNickLen = l
				}
			}
			maxNickLen = clamp(maxNickLen, 5, as.winW-64-10)
			sort.Slice(ab, func(i, j int) bool {
				ni := ab[i].Nick()
				nj := ab[j].Nick()
				return as.collator.CompareString(ni, nj) < 0
			})
			as.cwHelpMsgs(func(pf printf) {
				pf("")
				pf("Address Book")
				for _, entry := range ab {
					ignored := ""
					if entry.IsIgnored() {
						ignored = " (ignored)"
					}
					pf("%*s - %s%s", maxNickLen,
						strescape.Nick(entry.Nick()),
						entry.ID(), ignored)
				}
			})
			return nil
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return nickCompleter(arg, as)
			}
			if len(args) == 1 {
				if strings.HasPrefix("viewavatar", arg) {
					return []string{"viewavatar"}
				}
			}
			return nil
		},
	}, {
		cmd:     "msg",
		usage:   "<nick or id> <message>",
		aliases: []string{"m"},
		descr:   "Send a message to a known user",
		long:    []string{"Whenever two users with the same nick exist in the DB, disambiguate by using a prefix of the ID."},
		rawHandler: func(rawCmd string, args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "Nick or ID cannot be empty"}
			}
			if len(args) < 2 {
				return usageError{msg: "Message cannot be empty"}
			}

			ru, err := as.c.UserByNick(args[0])
			if err != nil {
				return err
			}

			_, msg := popNArgs(rawCmd, 2) // cmd + nick
			cw := as.findOrNewChatWindow(ru.ID(), ru.Nick())
			go as.pm(cw, msg)
			return nil
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return nickCompleter(arg, as)
			}
			return nil
		},
	}, {
		cmd:           "winclose",
		usableOffline: true,
		aliases:       []string{"wc"},
		descr:         "Close the current window",
		handler: func(args []string, as *appState) error {
			return as.closeActiveWindow()
		},
	}, {
		cmd:           "win",
		usableOffline: true,
		usage:         "<number | 'log' | 'console' | 'feed [user]'>",
		aliases:       []string{"w"},
		descr:         "Change the current window",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{"specify a window number"}
			}
			if args[0] == "log" {
				as.changeActiveWindow(activeCWLog)
			} else if args[0] == "lndlog" {
				as.changeActiveWindow(activeCWLndLog)
			} else if args[0] == "0" || args[0] == "console" {
				as.changeActiveWindow(activeCWDiag)
			} else if args[0] == "feed" {
				if len(args) > 1 {
					ru, err := as.c.UserByNick(args[1])
					if err != nil {
						return err
					}
					authorID := ru.ID()
					as.feedAuthor = &authorID
				} else {
					as.feedAuthor = nil
				}
				as.changeActiveWindow(activeCWFeed)
			} else {
				win, err := strconv.ParseInt(args[0], 10, 32)
				if err != nil {
					return err
				}

				// Chat windows are 0-based internally, but
				// 1-based to preserve legacy UX.
				win -= 1
				as.changeActiveWindow(int(win))
			}
			return nil
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) > 0 && args[0] == "feed" {
				return nickCompleter(arg, as)
			}
			return nil
		},
	}, {
		cmd:           "winlist",
		aliases:       []string{"wls"},
		usableOffline: true,
		descr:         "List currently opened windows",
		handler: func(args []string, as *appState) error {
			as.chatWindowsMtx.Lock()
			windows := as.chatWindows[:]
			as.chatWindowsMtx.Unlock()
			as.cwHelpMsgs(func(pf printf) {
				pf("Current windows")
				for i, cw := range windows {
					isGC := ""
					if cw.isGC {
						isGC = " (GC)"
					}
					pf("%2d - %s%s", i+1, cw.alias, isGC)
				}
			})
			return nil
		},
	}, {
		cmd:   "gc",
		usage: "[sub]",
		descr: "Group chat commands",
		sub:   gcCommands,
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return cmdCompleter(gcCommands, arg, false)
			}
			return nil
		},
		handler: subcmdNeededHandler,
	}, {
		cmd:   "invite",
		usage: "[sub]",
		descr: "invite commands",
		sub:   inviteCommands,
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return cmdCompleter(inviteCommands, arg, false)
			}
			return nil
		},
		handler: subcmdNeededHandler,
	}, {
		cmd:   "paytip",
		usage: "<nick or id> <dcr amount>",
		descr: "Send a tip with the given dcr amount to the user",
		long: []string{
			"Note: the tip is sent via LN, so the other peer only receives the tip if it is also online an connected to LN.",
		},
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "destination nick cannot be empty"}
			}
			if len(args) < 2 {
				return usageError{msg: "amount cannot be empty"}
			}

			uid, err := as.c.UIDByNick(args[0])
			if err != nil {
				return err
			}
			cw := as.findOrNewChatWindow(uid, args[0])
			dcrAmount, err := strconv.ParseFloat(args[1], 64)
			if err != nil {
				return err
			}

			go as.payTip(cw, dcrAmount)
			return nil
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return nickCompleter(arg, as)
			}
			return nil
		},
	}, {
		cmd:   "ft",
		usage: "[sub]",
		descr: "File Transfer commands",
		sub:   ftCommands,
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return cmdCompleter(ftCommands, arg, false)
			}
			return nil
		},
		handler: subcmdNeededHandler,
	}, {
		cmd:   "post",
		usage: "[sub]",
		descr: "post related commands",
		sub:   postCommands,
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return cmdCompleter(postCommands, arg, false)
			}
			return nil
		},
		handler: handleWithSubcmd(postCommands, "new"),
	}, {
		cmd:           "ln",
		usableOffline: true,
		usage:         "[sub]",
		descr:         "LN related commands",
		sub:           lnCommands,
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return cmdCompleter(lnCommands, arg, false)
			}
			return nil
		},
		handler: subcmdNeededHandler,
	}, {
		cmd:           "filters",
		usableOffline: true,
		usage:         "[sub]",
		descr:         "Content filter commands",
		sub:           filterCommands,
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return cmdCompleter(filterCommands, arg, false)
			}
			return nil
		},
		handler: subcmdNeededHandler,
	}, {
		cmd:     "rreset",
		aliases: []string{"rr", "ratchetreset"},
		usage:   "<nick>",
		descr:   "Request a ratchet reset with the given nick",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "nick cannot be empty"}
			}
			uid, err := as.c.UIDByNick(args[0])
			if err != nil {
				return err
			}
			cw := as.findOrNewChatWindow(uid, args[0])
			go as.requestRatchetReset(cw)
			return nil
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return nickCompleter(arg, as)
			}
			return nil
		},
	}, {
		cmd:   "rresetold",
		usage: "[age]",
		descr: "Request a ratchet reset with contacts with no messages since [age] ago",
		long: []string{"Request a ratchet reset with every remote user:",
			"1. From which no message has been received since [age] ago.",
			"2. No other KX reset procedure has been done since [age] ago.",
			"This command is useful to restablish comms after the local client has been offline for a long time (greater than the time the data lives on the server), on which case it's likely that many ratchets have been broken",
			"If [age] is not specified, it defaults to the ExpiryDays setting of the server.",
			"[age] may be specified either in days (without any suffix) or as a Go time.Duration string (with a time suffix)"},

		handler: func(args []string, as *appState) error {
			var interval time.Duration
			if len(args) > 0 {
				var err error
				interval, err = time.ParseDuration(args[0])
				if err != nil {
					d, err := strconv.ParseInt(args[0], 10, 64)
					if err != nil {
						return fmt.Errorf("arg %q is not a valid age",
							args[0])
					}
					interval = time.Duration(d) * 24 * time.Hour
				}
			} else {
				as.connectedMtx.Lock()
				expiry := as.expirationDays
				as.connectedMtx.Unlock()
				interval = time.Duration(expiry) * 24 * time.Hour
			}

			return as.resetAllOldRatchets(interval)
		},
	}, {
		cmd:     "mediateid",
		aliases: []string{"mi"},
		usage:   "<nick> <identity>",
		descr:   "Request nick to mediate an invite for the provided identity",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "nick cannot be empty"}
			}
			if len(args) < 2 {
				return usageError{msg: "identity cannot be empty"}
			}
			uid, err := as.c.UIDByNick(args[0])
			if err != nil {
				return err
			}
			var target clientintf.UserID
			if err := target.FromString(args[1]); err != nil {
				return err
			}
			cw := as.findOrNewChatWindow(uid, args[0])
			go as.requestMediateID(cw, target)
			return nil
		},
	}, {
		cmd:   "suggestkx",
		usage: "<invitee> <target>",
		descr: "Suggest 'invitee' to KX with 'target'",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "invitee cannot be empty"}
			}
			if len(args) < 2 {
				return usageError{msg: "target cannot be empty"}
			}
			invitee, err := as.c.UIDByNick(args[0])
			if err != nil {
				return err
			}
			target, err := as.c.UIDByNick(args[1])
			if err != nil {
				return err
			}

			go func() {
				err := as.c.SuggestKX(invitee, target)
				if err != nil {
					as.diagMsg("Unable to suggest %s KX with %s: %v",
						args[0], args[1], err)
				} else {
					as.diagMsg("Suggested %s KX with %s",
						args[0], args[1])
				}
			}()

			return nil
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 || len(args) == 1 {
				return nickCompleter(arg, as)
			}
			return nil
		},
	}, {
		cmd:     "treset",
		aliases: []string{"tr", "transreset"},
		usage:   "<mediator> <target>",
		descr:   "Request 'mediator' to mediate a ratchet reset with 'target'",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "mediator cannot be empty"}
			}
			if len(args) < 2 {
				return usageError{msg: "target cannot be empty"}
			}
			mediator, err := as.c.UIDByNick(args[0])
			if err != nil {
				return err
			}
			target, err := as.c.UIDByNick(args[1])
			if err != nil {
				return err
			}
			medCW := as.findOrNewChatWindow(mediator, args[0])
			targetCW := as.findOrNewChatWindow(target, args[1])
			go as.requestTransReset(medCW, targetCW)
			return nil
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 || len(args) == 1 {
				return nickCompleter(arg, as)
			}
			return nil
		},
	}, {
		cmd:           "reload",
		usableOffline: true,
		descr:         "Reload the configuration",
		handler: func(args []string, as *appState) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			theme, err := newTheme(cfg)
			if err != nil {
				return err
			}
			as.externalEditorForComments.Store(cfg.ExternalEditorForComments)
			as.mimeMap.Store(&cfg.MimeMap)
			as.styles.Store(theme)

			as.cwHelpMsg("reloaded configuration")
			return nil
		},
	}, {
		cmd:           "rename",
		usableOffline: true,
		descr:         "Modify the local nick of a user",
		usage:         "<old nick> <new nick>",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "old nick cannot be empty"}
			}
			if len(args) < 2 {
				return usageError{msg: "new nick cannot be empty"}
			}

			uid, err := as.c.UIDByNick(args[0])
			if err != nil {
				return err
			}

			newNick := args[1]
			err = as.c.RenameUser(uid, newNick)
			if err != nil {
				return err
			}

			as.cwHelpMsg("Renamed user %s to %q", uid, newNick)
			return nil
		},
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return nickCompleter(arg, as)
			}
			return nil
		},
	}, {
		cmd:           "rmpaystats",
		usableOffline: true,
		descr:         "Clear payment statistics",
		usage:         "<user | *>",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "nick cannot be empty"}
			}
			var uid *clientintf.UserID
			forUser := ""
			if args[0] != "*" {
				user, err := as.c.UIDByNick(args[0])
				if err != nil {
					return err
				}
				uid = &user
				forUser = fmt.Sprintf(" for user %q", args[0])
			}
			err := as.c.ClearPayStats(uid)
			if err != nil {
				return err
			}
			as.cwHelpMsg("Cleared payment stats%s", forUser)
			return nil
		},
	}, {
		cmd:           "info",
		usableOffline: true,
		descr:         "Show basic app version and information",
		handler: func(args []string, as *appState) error {
			var info *lnrpc.GetInfoResponse
			if as.lnRPC != nil {
				info, _ = as.lnRPC.GetInfo(as.ctx,
					&lnrpc.GetInfoRequest{})
			}

			as.cwHelpMsgs(func(pf printf) {
				pf("")
				pf("Client Information")
				pf("%s version %s", appName, version.String())
				pf("Client Identity: %s", as.c.PublicID())
				if info == nil {
					pf("Node does not have LN info")
				} else {
					pf("LN Node ID: %s", info.IdentityPubkey)
					pf("LN Software Version: %s", info.Version)
					if len(info.Chains) == 0 {
						pf("LN Node does not have chains")
					} else {
						pf("LN Network: %s", info.Chains[0].Network)
						pf("Block height: %d, hash %s", info.BlockHeight,
							info.BlockHash)
					}
				}
			})

			return nil
		},
	}, {
		cmd:           "skipwalletcheck",
		usableOffline: true,
		descr:         "Skip wallet check after connected to server",
		long: []string{"After connecting to the server, multiple checks are done to ensure the local client can make LN payments.",
			"Use this command to make the next time these checks are performed be skipped and allow the client to be used with the server regardless."},
		handler: func(args []string, as *appState) error {
			go as.skipNextWalletCheck()
			return nil
		},
	}, {
		cmd:           "pages",
		usableOffline: true,
		descr:         "Page and resource related commands",
		sub:           pagesCommands,
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return cmdCompleter(pagesCommands, arg, false)
			}
			return nil
		},
		handler: subcmdNeededHandler,
	}, {
		cmd:           "testcolor",
		usableOffline: true,
		descr:         "Test a color spec",
		usage:         "<attr>:<fg>:<bg>",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "color <attr>:<fg>:<bg> must be specified"}
			}
			style, err := colorDefnToLGStyle(args[0])
			if err != nil {
				return err
			}
			as.diagMsg(style.Render("On sangen hauskaa, että polkupyörä on maanteiden jokapäiväinen ilmiö."))
			return nil
		},
	}, {
		cmd:           "handshake",
		usableOffline: false,
		descr:         "Perform a 3-way handshake with an user",
		long:          []string{"This command is useful to check if the ratchet operations with the user are still working."},
		usage:         "<ID or nick>",
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return nickCompleter(arg, as)
			}
			return nil
		},
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{"user cannot be empty"}
			}

			ru, err := as.c.UserByNick(args[0])
			if err != nil {
				return err
			}

			err = as.c.Handshake(ru.ID())
			if err != nil {
				return err
			}

			as.cwHelpMsg("Starting 3-way handshake with %s",
				strescape.Nick(ru.Nick()))
			return nil
		},
	}, {
		cmd:           "setexchangerate",
		aliases:       []string{"setxchange"},
		usableOffline: true,
		usage:         "<USD/DCR> <USD/BTC>",
		descr:         "Manually set the exchange rate of USD/DCR and USD/BTC",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "USD/DCR rate cannot be empty"}
			}
			if len(args) < 2 {
				return usageError{msg: "USD/BTC rate cannot be empty"}
			}
			dcrPrice, err := strconv.ParseFloat(args[0], 64)
			if err != nil {
				return fmt.Errorf("invalid USD/DCR rate: %v", err)
			}
			btcPrice, err := strconv.ParseFloat(args[1], 64)
			if err != nil {
				return fmt.Errorf("invalid USD/BTC rate: %v", err)
			}
			as.cwHelpMsg("Setting manual exchange rate: DCR:%0.2f BTC:%0.2f",
				dcrPrice, btcPrice)
			as.rates.Set(dcrPrice, btcPrice)
			return nil
		},
	}, {
		cmd:           "myavatar",
		usableOffline: true,
		descr:         "Update the local client's avatar",
		sub:           myAvatarCmds,
		completer: func(args []string, arg string, as *appState) []string {
			if len(args) == 0 {
				return cmdCompleter(myAvatarCmds, arg, false)
			}
			return nil
		},
		handler: subcmdNeededHandler,
	}, {
		cmd:           "quit",
		usableOffline: true,
		descr:         "Quit the app",
		handler: func(args []string, as *appState) error {
			as.sendMsg(requestShutdown{})
			return nil
		},
	},
}

// findCommand returns the command, subcommand and rest of args for the given
// list of args.
func findCommand(args []string) (*tuicmd, *tuicmd, []string) {
	if len(args) == 0 {
		return nil, nil, nil
	}

	// Find command.
	var cmd *tuicmd
	for i := range commands {
		if commands[i].is(args[0]) {
			cmd = &commands[i]
			args = args[1:] // Pop cmd.
			break
		}
	}
	if len(args) == 0 || cmd == nil {
		return cmd, nil, args
	}

	// Find subcommand if it exists.
	for i := range cmd.sub {
		if cmd.sub[i].is(args[0]) {
			// Found sub!
			return cmd, &cmd.sub[i], args[1:]
		}
	}
	return cmd, nil, args
}

func genCompleterOpts(cl string, as *appState) []string {
	if cl == string(leader) {
		// Special case: generate list of top-level commands as
		// completer options.
		return cmdCompleter(commands, "", true)
	}

	args := parseCommandLine(cl)
	if len(args) == 0 {
		return nil
	}

	// Generate completion.
	cmd, subCmd, newArgs := findCommand(args)
	if cmd == nil {
		if len(args) == 1 {
			// Still completing the top-level command.
			return cmdCompleter(commands, args[0], true)
		}
		return nil
	}
	if subCmd != nil {
		cmd = subCmd
	}

	if cmd.completer == nil {
		return nil
	}

	// Pop last arg to call completer()
	var lastArg string
	if len(newArgs) > 0 {
		lastArg = newArgs[len(newArgs)-1]
		newArgs = newArgs[0 : len(newArgs)-1]
	}

	return cmd.completer(newArgs, lastArg, as)
}

// helpCmd is defined separately because it needs access to the `commands` var.
var helpCmd = tuicmd{
	cmd:           "help",
	usableOffline: true,
	usage:         "<command> <subcommand>",
	descr:         "Get command line help for the given command/subcommand",
	handler: func(args []string, as *appState) error {
		if len(args) == 0 {
			as.cwHelpMsgs(func(pf printf) {
				pf("Type %shelp [cmd] for help. Available commands:",
					string(leader))
				pf("")
				padLen := maxCmdLen(commands)
				for _, cmd := range commands {
					pf("%[3]*[1]s - %[2]s", cmd.cmd, cmd.descr, padLen)
				}
			})
			return nil
		}

		cmd, subCmd, _ := findCommand(args)
		if cmd == nil {
			errMsg := fmt.Sprintf("command %q not found", args[0])
			return usageError{msg: errMsg}
		}

		if len(args) > 1 && subCmd == nil {
			as.cwHelpMsgs(func(pf printf) {
				pf("Subcommand %q not found for command %q", args[1],
					args[0])
				pf("Type %shelp %s for the list of accepted subcommands",
					string(leader), args[0])
			})
		}

		fullCmd := cmd.cmd
		if subCmd != nil {
			fullCmd += " " + subCmd.cmd
			cmd = subCmd
		}

		as.cwHelpMsgs(func(pf printf) {
			pf("")
			pf("Help for command %q", fullCmd)
			if len(cmd.aliases) > 0 {
				pf("Aliases: %s", strings.Join(cmd.aliases, ", "))
			}
			pf("Usage: %s%s %s", string(leader), fullCmd, cmd.usage)
			pf("")
			pf(cmd.descr)
			pf("")
			for _, l := range cmd.long {
				pf(l)
			}
			padLen := maxCmdLen(cmd.sub)
			if len(cmd.sub) > 0 {
				pf("Subcommands:")
				for _, sub := range cmd.sub {
					pf("  %[3]*[1]s - %[2]s", sub.cmd, sub.descr, padLen)
				}
			}
		})

		return nil
	},
}

const commandReExpr = `("[^"]+"|[^\s"]+)`

var commandRe *regexp.Regexp

// parseCommandLinePreserveQuotes splits the given s string into args,
// preserving the double-quote char in quoted arguments.
func parseCommandLinePreserveQuotes(s string) []string {
	if len(s) == 0 || s[0] != leader {
		return nil
	}

	matches := commandRe.FindAllString(s[1:], -1)
	res := make([]string, len(matches))
	for i := range matches {
		s := strings.TrimSpace(matches[i])
		res[i] = s
	}
	return res
}

// parseCommandLine splits the given string into arguments separated by a
// space. The initial and final double-quote characters in quoted arguments are
// removed.
func parseCommandLine(s string) []string {
	res := parseCommandLinePreserveQuotes(s)
	for i, s := range res {
		// Unquote arg.
		if s[0] == '"' && s[len(s)-1] == '"' {
			s = s[1 : len(s)-1]
			res[i] = s
		}
	}
	return res
}

// popNArgs returns the N first arguments in s and the rest of the s string
// after those first n args.
//
// Returns nil if there aren't enough args.
func popNArgs(s string, n int) ([]string, string) {
	if len(s) == 0 {
		return nil, ""
	}

	if n <= 0 {
		return nil, ""
	}

	matches := commandRe.FindAllStringIndex(s, n)
	if len(matches) < n {
		return nil, ""
	}

	res := make([]string, n)
	for i, match := range matches {
		r := s[match[0]:match[1]]
		r = strings.TrimSpace(r)
		// Unquote arg.
		if r[0] == '"' && r[len(r)-1] == '"' {
			r = r[1 : len(r)-1]
		}
		res[i] = r
	}
	end := matches[len(matches)-1][1]
	rest := strings.TrimSpace(s[end:])
	return res, rest
}

func init() {
	// Parse command regexp.
	var err error
	commandRe, err = regexp.Compile(commandReExpr)
	if err != nil {
		panic(err)
	}

	commands = append(commands, helpCmd)

	// Sort commands.
	sort.Slice(commands, func(i, j int) bool {
		return commands[i].lessThan(&commands[j])
	})

	// Sort subcommands.
	for i := range commands {
		if len(commands[i].sub) < 2 {
			continue
		}

		sort.Slice(commands[i].sub, func(k, l int) bool {
			return commands[i].sub[k].lessThan(&commands[i].sub[l])
		})
	}
}
