package main

import (
	"fmt"
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
	"github.com/decred/dcrlnd/lnwire"
	"github.com/mitchellh/go-homedir"
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

// subcmdNeededHandler is used on top-level commands that only work with a
// subcommand.
func subcmdNeededHandler(args []string, as *appState) error {
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
			eRate := as.exchangeRate()
			as.cwHelpMsg(fmt.Sprintf("DCR: %.2f\tBTC: %.2f\t (USD/coin)", eRate.DCRPrice, eRate.BTCPrice))
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
						pf("(no data)")
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
							as.styles.help.Render(uid.String()))
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
			as.gcInvitesMtx.Lock()
			iid, ok := as.gcInvites[gcName]
			as.gcInvitesMtx.Unlock()
			if !ok {
				// Try to find it in the db.
				invites, err := as.c.ListGCInvitesFor(nil)
				if err != nil {
					return err
				}

				for i := len(invites) - 1; i >= 0; i-- {
					if invites[i].Invite.Name == gcName && !invites[i].Accepted {
						iid = invites[i].ID
						ok = true
						break
					}
				}

				if !ok {
					return fmt.Errorf("unrecognized gc invite %q", gcName)
				}
			}

			go func() {
				err := as.c.AcceptGroupChatInvite(iid)
				if err != nil {
					as.diagMsg("Unable to join gc %q: %v",
						gcName, err)
				} else {
					as.diagMsg("Accepting invitation to join gc %q", gcName)
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
				maxNameLen = clamp(maxNameLen, 5, as.winW-64-10)

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
							gcAlias,
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
			members := gc.Members[:]
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
				pf("Members of GC %q - %s", gcName, gc.ID.String())
				firstUknown := true
				for _, uid := range members {
					var ignored string
					if gcbl.IsBlocked(uid) {
						ignored = " (in GC blocklist)"
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
			return fileCompleter(arg)
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
			sf, _, err := as.c.ShareFile(filename, uid, atomCost, false, "")
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
	},
}

var postCommands = []tuicmd{
	{
		cmd:   "new",
		usage: "[<post>]",
		descr: "Create a new post",
		long:  []string{"If called without arguments, opens the create post window. Otherwise, it creates the given post."},
		handler: func(args []string, as *appState) error {
			as.sendMsg(showNewPostWindow{})
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
		descr:         "Create a new standard P2PKH address from the LN wallet",
		handler: func(args []string, as *appState) error {
			if as.lnRPC == nil {
				return fmt.Errorf("LN client not configured")
			}
			na, err := as.lnRPC.NewAddress(as.ctx,
				&lnrpc.NewAddressRequest{Type: lnrpc.AddressType_PUBKEY_HASH})
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

			chanPoint, err := strToChanPoint(args[0])
			if err != nil {
				return err
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

				pf("Pending Force-Closed Channels")
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
				}

				pf("Waiting Close Confirmation")
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

			msg := fmt.Sprintf("Channel Balance: %.8f, Max Inbound: %.8f, "+
				"Max Outbound: %.8f", dcrutil.Amount(bal.Balance).ToCoin(),
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
		usableOffline: true,
		descr:         "Show list of active channels",
		handler: func(args []string, as *appState) error {
			if as.lnRPC == nil {
				return fmt.Errorf("LN client not configured")
			}

			chans, err := as.lnRPC.ListChannels(as.ctx,
				&lnrpc.ListChannelsRequest{})
			if err != nil {
				return err
			}

			as.cwHelpMsgs(func(pf printf) {
				pf("LN Channels: %d", len(chans.Channels))
				for _, c := range chans.Channels {
					active := "✓"
					if !c.Active {
						active = "✗"
					}
					sid := lnwire.NewShortChanIDFromInt(c.ChanId)
					pf("  %scp:%s sid:%s cap:%.8f", active,
						c.ChannelPoint,
						sid,
						dcrutil.Amount(c.Capacity).ToCoin())
					pf("    %s   localBal:%.8f remoteBal:%.8f",
						c.RemotePubkey,
						dcrutil.Amount(c.LocalBalance).ToCoin(),
						dcrutil.Amount(c.RemoteBalance).ToCoin())
					pf("    unsettled:%.8f updts:%d htlcs:%d",
						dcrutil.Amount(c.UnsettledBalance).ToCoin(),
						c.NumUpdates,
						len(c.PendingHtlcs))
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

			as.cwHelpMsg("Attempting to pay invoice")
			go func() {
				pc, err := as.lnRPC.SendPayment(as.ctx)
				if err != nil {
					as.cwHelpMsg("PC: %v", err)
					return
				}

				req := &lnrpc.SendRequest{
					PaymentRequest: args[0],
				}
				err = pc.Send(req)
				if err != nil {
					as.cwHelpMsg("Unable to start payment: %v", err)
				}
				for res, err := pc.Recv(); ; {
					if err != nil {
						as.cwHelpMsg("PC receive error: %v", err)
						return
					}
					if res.PaymentError != "" {
						as.cwHelpMsg("Payment error: %s", res.PaymentError)
						return
					}
					as.cwHelpMsg("Payment done!")
					return
				}

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
		usage:         "<DCR amount> <dest address>",
		descr:         "Send funds from the on-chain wallet",
		usableOffline: true,
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "amount cannot be empty"}
			}
			if len(args) < 2 {
				return usageError{msg: "destination address cannot be empty"}
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
					Addr:   addr,
					Amount: int64(amount),
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
	},
}

var commands = []tuicmd{
	{
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
			go as.block(cw, uid)
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
	},
	{
		cmd:   "invite",
		usage: "<filename>",
		descr: "Create invitation file to send OOB to another user",
		handler: func(args []string, as *appState) error {
			if len(args) < 1 {
				return usageError{msg: "filename must be specified"}
			}

			filename, err := homedir.Expand(args[0])
			if err != nil {
				return err
			}
			go as.writeInvite(filename)
			return nil
		},
		completer: func(args []string, arg string, as *appState) []string {
			return fileCompleter(arg)
		},
	}, {
		cmd:   "add",
		usage: "<filename>",
		descr: "Accept the invite in the given file",
		completer: func(args []string, arg string, as *appState) []string {
			return fileCompleter(arg)
		},
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
		cmd:           "addressbook",
		usage:         "[<user>]",
		usableOffline: true,
		aliases:       []string{"ab"},
		descr:         "Show the address book of known remote users (or a user profile)",
		handler: func(args []string, as *appState) error {
			if len(args) > 0 {
				ru, err := as.c.UserByNick(args[0])
				if err != nil {
					return err
				}

				as.cwHelpMsgs(func(pf printf) {
					pii := ru.PublicIdentity()
					r := ru.RatchetDebugInfo()
					pf("")
					pf("Info for user %s", strescape.Nick(ru.Nick()))
					pf("              UID: %s", ru.ID())
					pf("             Name: %s", strescape.Content(pii.Name))
					pf("          Ignored: %v", ru.IsIgnored())
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
				})
				return nil
			}

			ab := as.c.AddressBook()
			var maxNickLen int
			for i := range ab {
				ab[i].Nick = strescape.Nick(ab[i].Nick)
				l := lipgloss.Width(ab[i].Nick)
				if l > maxNickLen {
					maxNickLen = l
				}
			}
			maxNickLen = clamp(maxNickLen, 5, as.winW-64-10)
			sort.Slice(ab, func(i, j int) bool {
				ni := ab[i].Nick
				nj := ab[j].Nick
				return as.collator.CompareString(ni, nj) < 0
			})
			as.cwHelpMsgs(func(pf printf) {
				pf("")
				pf("Address Book")
				for _, entry := range ab {
					ignored := ""
					if entry.Ignored {
						ignored = " (ignored)"
					}
					pf("%*s - %s%s", maxNickLen, entry.Nick,
						entry.ID, ignored)
				}
			})
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
		usage:         "<number | 'log' | 'console' | 'feed'>",
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
				return cmdCompleter(ftCommands, arg, false)
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
