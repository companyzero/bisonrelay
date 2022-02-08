import 'dart:convert';
import 'dart:typed_data';
// import 'package:dart_vlc/dart_vlc.dart' as vlc;
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/models/downloads.dart';
import 'package:flutter/material.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:markdown/markdown.dart' as md;
import 'package:provider/provider.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:bruig/theme_manager.dart';

class DownloadSource {
  final String uid;

  DownloadSource(this.uid);
}

class VideoInlineSyntax extends md.InlineSyntax {
  /// This is a primitive example pattern
  VideoInlineSyntax({
    String pattern = r'--video\[(.*?)\]--',
  }) : super(pattern);

  @override
  bool onMatch(md.InlineParser parser, Match match) {
    final videoURL = match.group(1);

    md.Element el = md.Element.text("video", videoURL!.toString());

    parser.addNode(el);
    return true;
  }
}

class ImageInlineSyntax extends md.InlineSyntax {
  /// This is a primitive example pattern
  ImageInlineSyntax({
    String pattern = r'--image\[(.*?)\]--',
  }) : super(pattern);

  @override
  bool onMatch(md.InlineParser parser, Match match) {
    final imageURL = match.group(1);

    md.Element el = md.Element.text("image", imageURL!.toString());

    parser.addNode(el);
    return true;
  }
}

class EmbedInlineSyntax extends md.InlineSyntax {
  /// This is a primitive example pattern
  EmbedInlineSyntax({
    String pattern = r'--embed\[(.*?)\]--',
  }) : super(pattern);

  @override
  bool onMatch(md.InlineParser parser, Match match) {
    final Map<String, String> parms = {};
    final rawParms = match.group(1) ?? "";
    rawParms.split(",").forEach((element) {
      var p = element.indexOf("=");
      if (p == -1) return;
      parms[element.substring(0, p)] = element.substring(p + 1);
    });

    // Only accept valid download FIDs.
    var download = parms["download"] ?? "";
    if (!RegExp(r"^[0-9a-fA-F]{64}$").hasMatch(download)) {
      download = "";
    }

    // URL-decode alt text.
    var alt = parms["alt"] ?? "";
    if (alt != "") {
      try {
        alt = Uri.decodeComponent(alt);
      } catch (exception) {
        // Ignore decoding errors and just print a debug msg.
        debugPrint("Unable to decode alt: $exception");
      }
    }

    var data = parms["data"] ?? "";

    // Bare link without embedded data.
    if (data == "" && download != "") {
      var el = md.Element.text(
          "download", alt != "" ? alt : "Download file $download");
      el.attributes["fid"] = download;
      parser.addNode(el);
      return true;
    }

    // Otherwise, we need data.
    if (data == "") {
      return true;
    }

    var tag = "";
    switch (parms["type"]) {
      case "image/jpeg":
        tag = "image";
        break;
      case "image/png":
      case "image/gif":
      case "image/bmp":
        tag = "image";
        break;
      case "text/plain":
        // Decode plain text directly.
        tag = "p";
        try {
          data = const Base64Decoder().convert(data).toString();
        } catch (exception) {
          data = "Unable to decode plain text contents: $exception";
        }
        break;
      default:
        return true;
    }
    md.Element el = md.Element.text(tag, data);

    if (download != "") {
      el.attributes["fid"] = download;
    }
    if (alt != "") {
      el.attributes["alt"] = alt;
    }

    parser.addNode(el);
    return true;
  }
}

/*
class _VideoMarkdownDesktopElement extends StatefulWidget {
  final String filename;
  _VideoMarkdownDesktopElement(this.filename, {Key? key}) : super(key: key);

  @override
  __VideoMarkdownDesktopElementState createState() =>
      __VideoMarkdownDesktopElementState();
}


class __VideoMarkdownDesktopElementState
    extends State<_VideoMarkdownDesktopElement> {
  vlc.Player player = vlc.Player(id: 69420);
  vlc.Media? media;

  @override
  void initState() {
    super.initState();
    media = vlc.Media.file(File(widget.filename));
    if (media != null) {
      player.open(media!);
    }
  }

  @override
  void dispose() {
    player.stop();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return vlc.Video(
      player: player,
      width: 320,
      height: 200,
    );
  }
}

class _VideoMarkdownMobileElement extends StatefulWidget {
  final String filename;
  _VideoMarkdownMobileElement(this.filename, {Key? key}) : super(key: key);

  @override
  __VideoMarkdownMobileElementState createState() =>
      __VideoMarkdownMobileElementState();
}

class __VideoMarkdownMobileElementState
    extends State<_VideoMarkdownMobileElement> {
  mbv.VideoPlayerController? controller;

  void initController() async {
    var f = File(widget.filename);
    var newController = await mbv.VideoPlayerController.file(f);
    await newController.initialize();
    mounted
        ? setState(() {
            controller = newController;
            controller?.play();
          })
        : null;
  }

  @override
  void initState() {
    super.initState();
    initController();
  }

  @override
  void dispose() {
    controller?.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    if (controller == null) {
      return Container(
        color: theme.cardColor,
        child: Center(
          child: Text("Loading..."),
        ),
      );
    }

    return AspectRatio(
        aspectRatio: controller!.value.aspectRatio,
        child: mbv.VideoPlayer(controller!));
  }
}

class VideoMarkdownElementBuilder extends MarkdownElementBuilder {
  final String basedir;
  VideoMarkdownElementBuilder(this.basedir);

  @override
  Widget visitElementAfter(md.Element element, TextStyle? preferredStyle) {
    final bool useVLC =
        Platform.isWindows || Platform.isLinux || Platform.isMacOS;

    // Protect against trying to fetch from !basedir.
    String filename = p.canonicalize(p.join(this.basedir, element.textContent));
    if (!p.isWithin(basedir, filename)) {
      return Container(color: Colors.amber, width: 100, height: 100);
    }

    return Container(
      margin: EdgeInsets.symmetric(horizontal: 0, vertical: 2),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.all(Radius.circular(6)),
      ),
      child: Padding(
          padding: const EdgeInsets.all(4.0),
          child: useVLC
              ? _VideoMarkdownDesktopElement(filename)
              : _VideoMarkdownMobileElement(filename)),
    );
  }
}
*/
class MarkdownArea extends StatelessWidget {
  final String text;
  const MarkdownArea(this.text, {Key? key}) : super(key: key);

  Future<void> launchUrlAwait(url) async {
    if (!await launchUrl(Uri.parse(url))) {
      throw 'Could not launch $url';
    }
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var darkTextColor = theme.indicatorColor;
    var textColor = theme.focusColor;
    var styleSheet = MarkdownStyleSheet(
      p: TextStyle(
          color: textColor, fontWeight: FontWeight.w300, letterSpacing: 0.44),
      h1: TextStyle(color: textColor),
      h2: TextStyle(color: textColor),
      h3: TextStyle(color: textColor),
      h4: TextStyle(color: textColor),
      h5: TextStyle(color: textColor),
      h6: TextStyle(color: textColor),
      em: TextStyle(color: textColor),
      strong: TextStyle(color: textColor),
      del: TextStyle(color: textColor),
      listBullet: TextStyle(color: textColor),
      blockquote: TextStyle(color: textColor),
      checkbox: TextStyle(color: textColor),
      tableBody: TextStyle(color: textColor),
      tableHead: TextStyle(color: textColor),
      blockquoteDecoration: BoxDecoration(color: darkTextColor),
      codeblockDecoration: BoxDecoration(color: darkTextColor),
      code: TextStyle(color: textColor),
    );
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => MarkdownBody(
            styleSheet: styleSheet,
            selectable: true,
            data: text.trim(),
            extensionSet: md.ExtensionSet(
                md.ExtensionSet.gitHubFlavored.blockSyntaxes, [
              md.EmojiSyntax(),
              ...md.ExtensionSet.gitHubFlavored.inlineSyntaxes
            ]),
            builders: {
              //"video": VideoMarkdownElementBuilder(basedir),
              "image": ImageMarkdownElementBuilder(),
              "download":
                  DownloadLinkElementBuilder(TextStyle(color: darkTextColor)),
            },
            onTapLink: (text, url, blah) {
              launchUrlAwait(url);
            },
            inlineSyntaxes: [
              //VideoInlineSyntax(),
              //ImageInlineSyntax()
              EmbedInlineSyntax(),
            ]));
  }
}

class Downloadable extends StatelessWidget {
  final String tip;
  final String fid;
  final Widget child;
  const Downloadable(this.tip, this.fid, this.child, {Key? key})
      : super(key: key);

  void download(BuildContext context) async {
    var downloads = Provider.of<DownloadsModel>(context, listen: false);
    var source = Provider.of<DownloadSource>(context, listen: false);
    try {
      await downloads.getUnknownUserFile(source.uid, fid);
      showSuccessSnackbar(context, "Added $fid to download queue");
    } catch (exception) {
      showErrorSnackbar(context, "Unable to start download: $exception");
    }
  }

  @override
  Widget build(BuildContext context) => Tooltip(
      message: tip,
      child: InkWell(
          hoverColor: Theme.of(context).hoverColor,
          onTap: fid != "" ? () => download(context) : null,
          child: Container(
            margin: const EdgeInsets.symmetric(horizontal: 2, vertical: 2),
            child: child,
          )));
}

class DownloadLinkElementBuilder extends MarkdownElementBuilder {
  final TextStyle style;

  DownloadLinkElementBuilder(this.style);

  @override
  Widget visitElementAfter(md.Element element, TextStyle? preferredStyle) {
    var download = element.attributes["fid"] ?? "";
    var tip = "Click to download file $download";
    return Downloadable(
        tip,
        download,
        Text(
          element.textContent,
          style: style,
        ));
  }
}

class ImageMarkdownElementBuilder extends MarkdownElementBuilder {
  @override
  Widget visitElementAfter(md.Element element, TextStyle? preferredStyle) {
    Uint8List imgBytes;
    try {
      imgBytes = const Base64Decoder().convert(element.textContent);
    } catch (exception) {
      return Text("Unable to decode image: $exception");
    }

    var alt = element.attributes["alt"] ?? "";
    var download = element.attributes["fid"] ?? "";
    var tip = "";
    if (alt != "") {
      tip = alt;
      if (download != "") {
        tip += "\n\n";
      }
    }
    if (download != "") {
      tip += "Click to download file $download";
    }

    Image img;
    try {
      img = Image.memory(imgBytes);
    } catch (exception) {
      print("Unable to decode image: $exception");
      img = Image.asset("assets/images/invalidimg.png");
    }

    return Downloadable(tip, download, img);
  }
}
