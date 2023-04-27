package types

// Services returns the full list of service definitions defined in this
// package.
func Services() []ServiceDefn {
	return []ServiceDefn{VersionServiceDefn(), ChatServiceDefn(),
		PostsServiceDefn(), PaymentsServiceDefn(), GCServiceDefn(),
		ResourcesServiceDefn()}
}

// HelpForMessage returns the top-level help defined for the given proto
// message.
func HelpForMessage(name string) string {
	return help_messages[name]["@"]
}

// HelpForMessageField returns the help message defined for the specified
// message field.
func HelpForMessageField(message, field string) string {
	return help_messages[message][field]
}
