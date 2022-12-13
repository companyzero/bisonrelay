package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type theme struct {
	header         lipgloss.Style
	footer         lipgloss.Style
	footerMention  lipgloss.Style
	edit           lipgloss.Style
	focused        lipgloss.Style
	blurred        lipgloss.Style
	cursor         lipgloss.Style
	noStyle        lipgloss.Style
	help           lipgloss.Style
	cursorModeHelp lipgloss.Style
	timestamp      lipgloss.Style
	timestampHelp  lipgloss.Style
	nick           lipgloss.Style
	nickMe         lipgloss.Style
	nickGC         lipgloss.Style
	msg            lipgloss.Style
	unsent         lipgloss.Style
	online         lipgloss.Style
	offline        lipgloss.Style
	checkingWallet lipgloss.Style
	err            lipgloss.Style
	mention        lipgloss.Style
	embed          lipgloss.Style

	blink bool
}

func textToColor(in string) (lipgloss.Color, error) {
	var c lipgloss.Color
	switch strings.ToLower(in) {
	case "na":
	case "black":
		c = "0" // ttk.ColorBlack
	case "red":
		c = "1" // ttk.ColorRed
	case "green":
		c = "2" // ttk.ColorGreen
	case "yellow":
		c = "3" // ttk.ColorYellow
	case "blue":
		c = "4" // ttk.ColorBlue
	case "magenta":
		c = "5" // ttk.ColorMagenta
	case "cyan":
		c = "6" // ttk.ColorCyan
	case "white":
		c = "7" // ttk.ColorWhite
	default:
		return c, fmt.Errorf("invalid color: %v", in)
	}
	return c, nil
}

// colorDefnToLGStyle converts a color definition used in the config files to a
// lipgloss style.
func colorDefnToLGStyle(color string) (lipgloss.Style, error) {
	s := strings.Split(color, ":")
	style := lipgloss.NewStyle()
	if len(s) != 3 {
		return style, fmt.Errorf("invalid color format: " +
			"attribute:foreground:background")
	}

	aa := strings.Split(strings.ToLower(s[0]), ",")
	for _, k := range aa {
		switch strings.ToLower(k) {
		case "bold":
			style = style.Bold(true)
		case "underline":
			style = style.Underline(true)
		case "reverse":
			style = style.Reverse(true)
		default:
			return style, fmt.Errorf("invalid attribute: %v", k)
		}
	}

	fg, err := textToColor(s[1])
	if err != nil {
		return style, err
	}
	style.Foreground(fg)

	bg, err := textToColor(s[2])
	if err != nil {
		return style, err
	}
	style.Background(bg)

	return style, nil
}

func newTheme(args *config) (*theme, error) {
	var nickMe, nick, nickGC lipgloss.Style
	var blink bool
	var err error

	if args != nil {
		nickMe, err = colorDefnToLGStyle(args.NickColor)
		if err != nil {
			return nil, err
		}
		nick, err = colorDefnToLGStyle(args.PMOtherColor)
		if err != nil {
			return nil, err
		}
		nickGC, err = colorDefnToLGStyle(args.GCOtherColor)
		if err != nil {
			return nil, err
		}
		blink = args.BlinkCursor
	} else {
		nickMe = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("7"))
		nick = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("6"))
		nickGC = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("2"))
		blink = true
	}

	return &theme{
		// Theme
		header: lipgloss.NewStyle().
			Bold(false).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#000044")),

		footer: lipgloss.NewStyle().
			Bold(false).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#000044")),
		footerMention: lipgloss.NewStyle().
			Background(lipgloss.Color("#000044")).
			Foreground(lipgloss.Color("5")).Bold(true),

		edit: lipgloss.NewStyle().
			Bold(false).
			Foreground(lipgloss.Color("#aaaaaa")).
			Background(lipgloss.Color("#000000")),

		timestamp: lipgloss.NewStyle().
			Bold(false).
			Foreground(lipgloss.Color("#a1ba22")),
		timestampHelp: lipgloss.NewStyle().
			Bold(false).
			Foreground(lipgloss.Color("#6b6b6b")),

		/*
			nick: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#22a1ba")),
			nickMe: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#22ba3b")),
		*/
		nick:   nick,
		nickMe: nickMe,
		nickGC: nickGC,
		mention: lipgloss.NewStyle().
			Foreground(lipgloss.Color("5")).Bold(true),

		msg:    lipgloss.NewStyle(),
		unsent: lipgloss.NewStyle().Foreground(lipgloss.Color("240")),

		online:         lipgloss.NewStyle().Foreground(lipgloss.Color("154")),
		offline:        lipgloss.NewStyle().Foreground(lipgloss.Color("160")),
		checkingWallet: lipgloss.NewStyle().Foreground(lipgloss.Color("214")),

		err: lipgloss.NewStyle().Foreground(lipgloss.Color("160")).Bold(true),

		focused:        lipgloss.NewStyle().Foreground(lipgloss.Color("205")),
		blurred:        lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		cursor:         lipgloss.NewStyle().Foreground(lipgloss.Color("205")),
		noStyle:        lipgloss.NewStyle(),
		help:           lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		cursorModeHelp: lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		embed:          lipgloss.NewStyle().Foreground(lipgloss.Color("27")),

		blink: blink,
	}, nil
}

// renderPF captures `style` and returns a new printf-like function that uses
// style to render the string.
func renderPF(style lipgloss.Style) func(string, ...interface{}) string {
	return func(format string, args ...interface{}) string {
		return style.Render(fmt.Sprintf(format, args...))
	}
}
