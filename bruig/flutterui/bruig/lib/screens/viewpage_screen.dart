import 'dart:convert';

import 'package:bruig/components/containers.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/md_elements.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/resources.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

class ViewPagesScreenTitle extends StatelessWidget {
  const ViewPagesScreenTitle({super.key});

  @override
  Widget build(BuildContext context) {
    return const Txt.L("Pages");
  }
}

class _ActivePageScreen extends StatefulWidget {
  final PagesSession session;
  final ClientModel client;
  const _ActivePageScreen(this.session, this.client);

  @override
  State<_ActivePageScreen> createState() => _ActivePageScreenState();
}

class _ActivePageScreenState extends State<_ActivePageScreen> {
  PagesSession get session => widget.session;
  String markdownData = "";
  Key pageKey = UniqueKey();

  void updateSession() {
    var newMdData = session.pageData();
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
    if (session.currentPage == null) {
      return Container(
        padding: const EdgeInsets.all(16),
        child: const Text("Loading page..."),
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
      padding: const EdgeInsets.all(16),
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Text("Page Source: $nick (${page.uid})"),
        const SizedBox(height: 10),
        Text("Page Path: $loading $path"),
        const SizedBox(height: 10),
        const Divider(),
        const SizedBox(height: 10),
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
    var snackbar = SnackBarModel.of(context);

    try {
      var sess = await resources.fetchPage(uid, path, 0, 0, null, "");
      resources.mostRecent = sess;
    } catch (exception) {
      snackbar.error("Unable to fetch local page: $exception");
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
    var activeSess = resources.mostRecent;
    return Row(children: [
      SecondarySideMenuList(
          width: 125,
          list: ListView.builder(
              itemCount: sessions.length,
              itemBuilder: (BuildContext context, int index) {
                var selected = activeSess == sessions[index];
                return SecondarySideMenuItem(ListTile(
                  title: Txt.S("Session ${sessions[index].id}"),
                  selected: selected,
                  onTap: () {
                    resources.mostRecent = sessions[index];
                  },
                ));
              }),
          footer: Row(children: [
            IconButton(
              onPressed: viewLocal,
              icon: const Icon(Icons.browser_updated_sharp),
              tooltip: "Open local pages",
            )
          ])),
      activeSess != null
          ? Expanded(child: _ActivePageScreen(activeSess, widget.client))
          : const Empty(),
    ]);
  }
}
