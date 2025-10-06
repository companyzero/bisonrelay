package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/companyzero/bisonrelay/clientrpc/types"
	"github.com/decred/slog"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const appName = "brcrpc"

var errCmdDone = errors.New("command done")

type protoMessageValue struct {
	m protoreflect.Message
	f protoreflect.FieldDescriptor
}

func (pmv protoMessageValue) String() string {
	return ""
}

func (pmv protoMessageValue) Set(s string) error {
	kind := pmv.f.Kind()
	switch kind {
	case protoreflect.StringKind:
		pmv.m.Set(pmv.f, protoreflect.ValueOfString(s))
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Fixed32Kind, protoreflect.Sfixed32Kind:
		v, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return err
		}
		pmv.m.Set(pmv.f, protoreflect.ValueOfInt32(int32(v)))

	case protoreflect.Uint32Kind:
		v, err := strconv.ParseUint(s, 10, 32)
		if err != nil {
			return err
		}
		pmv.m.Set(pmv.f, protoreflect.ValueOfUint32(uint32(v)))

	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Fixed64Kind, protoreflect.Sfixed64Kind:
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		pmv.m.Set(pmv.f, protoreflect.ValueOfInt64(v))

	case protoreflect.Uint64Kind:
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return err
		}
		pmv.m.Set(pmv.f, protoreflect.ValueOfUint64(v))

	case protoreflect.DoubleKind:
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return err
		}
		pmv.m.Set(pmv.f, protoreflect.ValueOfFloat64(v))

	case protoreflect.BytesKind:
		pmv.m.Set(pmv.f, protoreflect.ValueOfBytes([]byte(s)))

	default:
		return fmt.Errorf("unsupported kind %s in field %s", kind, pmv.f.Name())
	}
	return nil
}

func allCommandNames() []string {
	res := make([]string, 0)
	for _, svc := range types.Services() {
		svcName := svc.Name
		for methodName := range svc.Methods {
			cmdName := svcName + "." + methodName
			res = append(res, cmdName)
		}
	}
	return res
}

func findMethod(cmdName string) (*types.ServiceDefn, *types.MethodDefn) {
	for _, svc := range types.Services() {
		svcName := svc.Name
		for methodName, method := range svc.Methods {
			name := svcName + "." + methodName
			if cmdName == name {
				return &svc, &method
			}
		}
	}
	return nil, nil
}

func flattenFields(md protoreflect.MessageDescriptor) [][]protoreflect.FieldDescriptor {
	var stack [][]protoreflect.FieldDescriptor
	var res [][]protoreflect.FieldDescriptor
	fields := md.Fields()
	for i := 0; i < fields.Len(); i++ {
		stack = append(stack, []protoreflect.FieldDescriptor{fields.Get(i)})
	}

	for len(stack) > 0 {
		l := len(stack)
		item := stack[l-1]
		stack = stack[:l-1]
		last := item[len(item)-1]
		if last.Kind() == protoreflect.MessageKind {
			l := len(item)
			fields := last.Message().Fields()
			for i := 0; i < fields.Len(); i++ {
				newItem := append(item[:l:l], fields.Get(i))
				stack = append(stack, newItem)
			}
		} else {
			res = append(res, item)
		}
	}

	sort.Slice(res, func(i, j int) bool {
		li, lj := len(res[i]), len(res[j])
		min := li
		if lj < li {
			min = lj
		}
		for k := 0; k < min; k++ {
			if res[i][k].JSONName() < res[j][k].JSONName() {
				return true
			}
		}
		return false
	})

	return res
}

func fieldsToArgName(fields []protoreflect.FieldDescriptor) string {
	var res string
	for i := 0; i < len(fields)-1; i++ {
		res += fields[i].JSONName() + "."
	}
	res += fields[len(fields)-1].JSONName()
	return res
}

func fieldsTypeAndName(fields []protoreflect.FieldDescriptor) (string, string) {
	l := len(fields)
	item := fields[l-1]
	parent := string(item.Parent().Name())
	name := string(item.Name())
	return parent, name
}

func helpLinesForArg(helpMsg string, pad int) (string, []string) {
	lines := strings.Split(helpMsg, "\n")
	if len(lines) > 0 {
		helpMsg = lines[0]
		lines = lines[1:]
	}
	for i := range lines {
		lines[i] = strings.Repeat(" ", pad) + lines[i]
	}
	return helpMsg, lines
}

func commandUsage(cmdName string) error {
	_, method := findMethod(cmdName)
	if method == nil {
		return fmt.Errorf("command %q not found", cmdName)
	}

	pf := func(f string, args ...interface{}) {
		fmt.Fprintf(os.Stderr, f, args...)
		fmt.Fprintf(os.Stderr, "\n")
	}

	pf("Help for command %s", cmdName)
	pf("Description:")
	pf(method.Help)
	pf("")

	reqDefn := method.RequestDefn()
	if reqDefn.Fields().Len() == 0 {
		pf("No request arguments")
	} else {
		pf("Request arguments:")
	}

	argFields := flattenFields(reqDefn)
	for _, fields := range argFields {
		parent, name := fieldsTypeAndName(fields)
		helpMsg := types.HelpForMessageField(parent, name)
		arg := fieldsToArgName(fields)
		firstLine, rest := helpLinesForArg(helpMsg, len(arg)+5)
		pf("  -%s: %s", arg, firstLine)
		for _, line := range rest {
			pf(line)
		}
	}

	pf("")
	resDefn := method.ResponseDefn()
	if resDefn.Fields().Len() == 0 {
		pf("No response fields")
	} else {
		pf("Response fields:")
	}

	resFields := flattenFields(resDefn)
	for _, fields := range resFields {
		parent, name := fieldsTypeAndName(fields)
		helpMsg := types.HelpForMessageField(parent, name)
		arg := fieldsToArgName(fields)
		firstLine, rest := helpLinesForArg(helpMsg, len(arg)+5)
		pf("  %s: %s", arg, firstLine)
		for _, line := range rest {
			pf(line)
		}
	}

	return nil
}

func parseCmdFlags(cmdName string, args []string) (proto.Message, error) {
	_, method := findMethod(cmdName)
	if method == nil {
		return nil, fmt.Errorf("command %q not found", cmdName)
	}

	cmdCfg := flag.NewFlagSet(cmdName, flag.ContinueOnError)
	req := method.NewRequest()

	subFields := map[string]protoreflect.Message{}

	argFields := flattenFields(req.ProtoReflect().Descriptor())
	for _, fields := range argFields {
		var f protoreflect.FieldDescriptor
		m := req.ProtoReflect()
		var parentName string
		for i := range fields {
			if i == 0 {
				f = fields[i]
				continue
			}
			parentName += string(f.Name()) + "."
			if f.IsMap() {
				// TODO: support passing map arguments.
				continue
			}
			if v, ok := subFields[parentName]; !ok {
				newm := m.Get(f).Message().New()
				m.Set(f, protoreflect.ValueOf(newm))
				subFields[parentName] = newm
				m = newm
			} else {
				m = v
			}
			f = fields[i]
		}
		pmv := protoMessageValue{m: m, f: f}
		arg := fieldsToArgName(fields)
		cmdCfg.Var(pmv, arg, "")
	}

	if err := cmdCfg.Parse(args); err != nil {
		return nil, err
	}
	return req, nil
}

func responseProducer(cmdName string) (func() proto.Message, error) {
	_, method := findMethod(cmdName)
	if method == nil {
		return nil, fmt.Errorf("method %q not found", cmdName)
	}

	return method.NewResponse, nil
}

type config struct {
	cmdName        string
	req            proto.Message
	resProducer    func() proto.Message
	marshalOpts    protojson.MarshalOptions
	log            slog.Logger
	isStreaming    bool
	url            string
	serverCertPath string
	clientCertPath string
	clientKeyPath  string
}

func mainUsage(fs *flag.FlagSet) {
	pf := func(f string, args ...interface{}) {
		fmt.Fprintf(os.Stderr, f, args...)
		fmt.Fprintf(os.Stderr, "\n")
	}
	pf("Usage: %s [options] <command> [arguments]", appName)
	pf("General Options:")
	fs.PrintDefaults()

	pf("")
	pf("Available commands:")
	cmds := allCommandNames()
	for _, name := range cmds {
		pf("  %s", name)
	}
}

func loadConfig() (*config, error) {
	cfg := &config{
		log: slog.Disabled,
		marshalOpts: protojson.MarshalOptions{
			UseProtoNames: false,
		},
	}

	preCfg := flag.NewFlagSet(appName, flag.ContinueOnError)
	preCfg.Usage = func() {}
	flagDebugLevel := preCfg.String("debuglevel", "disabled", "Log level to stderr")
	flagURL := preCfg.String("url", "wss://127.0.0.1:7676/ws", "URL of the websocket endpoint")
	flagServerCertPath := preCfg.String("servercert", "~/.brclient/rpc.cert", "Path to rpc.cert file")
	flagClientCertPath := preCfg.String("clientcert", "~/.brclient/rpc-client.cert", "Path to rpc-client.cert file")
	flagClientKeyPath := preCfg.String("clientkey", "~/.brclient/rpc-client.key", "Path to rpc-client.key file")

	var args []string
	if len(os.Args) > 1 {
		args = os.Args[1:]
	}
	err := preCfg.Parse(args)

	// Help command.
	if errors.Is(err, flag.ErrHelp) {
		if preCfg.NArg() > 0 {
			cmdName := preCfg.Arg(0)
			if err := commandUsage(cmdName); err != nil {
				return nil, err
			}
			return nil, errCmdDone
		}
		mainUsage(preCfg)
		return nil, errCmdDone
	}

	// Other parsing errors.
	if err != nil {
		return nil, err
	}

	// Double check command was specified.
	if preCfg.NArg() == 0 {
		return nil, fmt.Errorf("no command specified")
	}

	// Setup general options.
	bknd := slog.NewBackend(os.Stderr)
	cfg.log = bknd.Logger("RPCC")
	switch *flagDebugLevel {
	case "error":
		cfg.log.SetLevel(slog.LevelError)
	case "warn":
		cfg.log.SetLevel(slog.LevelWarn)
	case "info":
		cfg.log.SetLevel(slog.LevelInfo)
	case "debug":
		cfg.log.SetLevel(slog.LevelDebug)
	case "trace":
		cfg.log.SetLevel(slog.LevelTrace)
	case "disabled":
		cfg.log = slog.Disabled
	}

	cfg.url = *flagURL
	cfg.serverCertPath = *flagServerCertPath
	cfg.clientCertPath = *flagClientCertPath
	cfg.clientKeyPath = *flagClientKeyPath

	// Figure out the command.
	cfg.cmdName = strings.TrimSpace(preCfg.Arg(0))
	_, method := findMethod(cfg.cmdName)
	if method == nil {
		return nil, fmt.Errorf("method %q not found", cfg.cmdName)
	}

	// Parse command args.
	cmdArgs := preCfg.Args()[1:]
	cfg.req, err = parseCmdFlags(cfg.cmdName, cmdArgs)
	if err != nil {
		return nil, err
	}

	cfg.resProducer, err = responseProducer(cfg.cmdName)
	if err != nil {
		return nil, err
	}
	cfg.isStreaming = method.IsStreaming

	return cfg, nil
}
