import 'dart:convert';
import 'dart:typed_data';

import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/md_elements.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/resources.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

class ViewPagesScreenTitle extends StatelessWidget {
  const ViewPagesScreenTitle({super.key});

  @override
  Widget build(BuildContext context) {
    const baseTitle = "Bison Relay / Pages";
    return const Text(baseTitle);
  }
}

class _ActivePageScreen extends StatefulWidget {
  final PagesSession session;
  final ClientModel client;
  const _ActivePageScreen(this.session, this.client, {super.key});

  @override
  State<_ActivePageScreen> createState() => _ActivePageScreenState();
}

class _ActivePageScreenState extends State<_ActivePageScreen> {
  PagesSession get session => widget.session;
  String markdownData = "";
  Key pageKey = UniqueKey();

  void updateSession() {
    var data = session.currentPage?.response.data ?? Uint8List(0);
    var newMdData = utf8.decode(data);
    newMdData += "\n";
    setState(() {
      if (newMdData != markdownData) {
        markdownData = newMdData;

        // Bump pageKey so that the Provider<PageSource> is recreated with the new
        // page. This is needed so that navigating pages across different UIDs
        // work.
        pageKey = UniqueKey();
      }
    });
  }

  @override
  void initState() {
    super.initState();
    updateSession();
    session.addListener(updateSession);
  }

  @override
  void didUpdateWidget(_ActivePageScreen oldWidget) {
    oldWidget.session.removeListener(updateSession);
    super.didUpdateWidget(oldWidget);
    session.addListener(updateSession);
    updateSession();
  }

  @override
  void dispose() {
    session.removeListener(updateSession);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    var backgroundColor = theme.backgroundColor;
    var ts = TextStyle(color: textColor);

    if (session.currentPage == null) {
      return Container(
        margin: const EdgeInsets.all(1),
        decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(3), color: backgroundColor),
        padding: const EdgeInsets.all(16),
        child: Text("Loading page", style: TextStyle(color: textColor)),
      );
    }

    var page = session.currentPage!;
    var path = page.request.path.join("/");
    var nick = widget.client.getNick(page.uid);

    var loading = "";
    if (session.loading) {
      loading = "⌛";
    }

    return Container(
      alignment: Alignment.topLeft,
      margin: const EdgeInsets.all(1),
      decoration: BoxDecoration(borderRadius: BorderRadius.circular(3)),
      padding: const EdgeInsets.all(16),
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Text("Page Source: $nick (${page.uid})", style: ts),
        const SizedBox(height: 10),
        Text("Page Path: $loading $path", style: ts),
        const SizedBox(height: 20),
        Expanded(
            child: ListView(children: [
          Provider<PagesSource>(
            key: pageKey,
            create: (context) =>
                PagesSource(page.uid, page.sessionID, page.pageID),
            builder: (context, child) => MarkdownArea(markdownData, false),
          ),
        ])),
      ]),
    );
  }
}

class ViewPageScreen extends StatefulWidget {
  static String routeName = "/pages";
  final ResourcesModel resources;
  final ClientModel client;
  const ViewPageScreen(this.resources, this.client, {super.key});

  @override
  State<ViewPageScreen> createState() => _ViewPageScreenState();
}

class _ViewPageScreenState extends State<ViewPageScreen> {
  final Utf8Codec utf8 = const Utf8Codec();
  ResourcesModel get resources => widget.resources;
  List<PagesSession> sessions = [];
  String markdownData = "";

  void updateSessions() {
    setState(() => sessions = resources.sessions);
  }

  void viewLocal() async {
    var uid = widget.client.publicID;
    var path = ["index.md"];
    try {
      var sess = await resources.fetchPage(uid, path, 0, 0);
      resources.mostRecent = sess;
    } catch (exception) {
      showErrorSnackbar(context, "Unable to fetch local page: $exception");
    }
  }

  @override
  void initState() {
    super.initState();
    updateSessions();
    resources.addListener(updateSessions);
  }

  @override
  void didUpdateWidget(ViewPageScreen oldWidget) {
    oldWidget.resources.removeListener(updateSessions);
    super.didUpdateWidget(oldWidget);
    resources.addListener(updateSessions);
  }

  @override
  void dispose() {
    resources.removeListener(updateSessions);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var unselectedTextColor = theme.dividerColor;
    var selectedTextColor = theme.focusColor; // MESSAGE TEXT COLOR
    var sidebarBackground = theme.backgroundColor;
    var hoverColor = theme.hoverColor;
    var tsUnselected = TextStyle(
        color: unselectedTextColor, fontSize: 11, fontWeight: FontWeight.w400);
    var tsSelected = TextStyle(
        color: selectedTextColor, fontSize: 11, fontWeight: FontWeight.w400);

    var activeSess = resources.mostRecent;

    return Row(children: [
      Container(
          width: 118,
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(5),
            gradient: LinearGradient(
                begin: Alignment.centerRight,
                end: Alignment.centerLeft,
                colors: [
                  hoverColor,
                  sidebarBackground,
                  sidebarBackground,
                ],
                stops: const [
                  0,
                  0.51,
                  1
                ]),
          ),
          child: Column(children: [
            Expanded(
                child: ListView.builder(
                    itemCount: sessions.length,
                    itemBuilder: (BuildContext context, int index) {
                      var selected = activeSess == sessions[index];
                      return ListTile(
                        title: Text("Session ${sessions[index].id}",
                            style: selected ? tsSelected : tsUnselected),
                        selected: selected,
                        onTap: () {
                          resources.mostRecent = sessions[index];
                        },
                      );
                    })),
            Row(children: [
              IconButton(
                onPressed: viewLocal,
                icon: const Icon(Icons.browser_updated_sharp),
                tooltip: "Open local pages",
              )
            ]),
          ])),
      activeSess != null
          ? Expanded(child: _ActivePageScreen(activeSess, widget.client))
          : const Empty(),
    ]);
  }
}
