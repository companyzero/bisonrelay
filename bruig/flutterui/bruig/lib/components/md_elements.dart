import 'dart:convert';
import 'dart:ui';
import 'dart:typed_data';
// import 'package:dart_vlc/dart_vlc.dart' as vlc;
import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/info_grid.dart';
import 'package:bruig/components/inputs.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/models/downloads.dart';
import 'package:bruig/models/resources.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:markdown/markdown.dart' as md;
import 'package:provider/provider.dart';
import 'package:tuple/tuple.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:bruig/theme_manager.dart';
import 'package:bruig/components/image_dialog.dart';

class DownloadSource {
  final String uid;

  DownloadSource(this.uid);
}

class PagesSource {
  final String uid;
  final int sessionID;
  final int pageID;

  PagesSource(this.uid, this.sessionID, this.pageID);
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

class _FormField {
  final String type;
  final String name;
  final String label;
  dynamic value;

  _FormField(this.type, {this.name = "", this.label = "", this.value});
}

class _FormElement extends md.Element {
  final List<_FormField> fields;

  _FormElement(this.fields) : super("form", [md.Text("")]);
}

class _FormBlockSyntax extends md.BlockSyntax {
  static String closeTag = r'--/form--';
  static RegExp tagPattern = RegExp(r'^--form--$');
  static RegExp fieldPattern = RegExp(r'([\w]+)="([^"]*)"');

  @override
  RegExp get pattern => tagPattern;

  @override
  bool canEndBlock(md.BlockParser parser) => parser.current == "--/form--";

  @override
  md.Node? parse(md.BlockParser parser) {
    parser.advance();

    List<_FormField> children = [];

    while (!parser.isDone && !md.BlockSyntax.isAtBlockEnd(parser)) {
      if (parser.current == closeTag) {
        parser.advance();
        continue;
      }

      var matches = fieldPattern.allMatches(parser.current.content);
      String type = "";
      Map<Symbol, dynamic> args = {};
      for (var m in matches) {
        if (m.groupCount < 2) {
          continue;
        }
        String name = m.group(1)!;
        String value = m.group(2)!;
        switch (name) {
          case "type":
            type = value;
            break;
          case "value":
          case "label":
          case "name":
            args[Symbol(name)] = value;
            break;
        }
      }

      _FormField field = Function.apply(_FormField.new, [type], args);
      children.add(field);
      parser.advance();
    }

    var res = md.Element("p", [_FormElement(children)]);
    return res;
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
  final bool hasNick;
  const MarkdownArea(this.text, this.hasNick, {Key? key}) : super(key: key);

  Future<void> launchUrlAwait(context, url) async {
    var parsed = Uri.parse(url);
    var downSource = Provider.of<DownloadSource?>(context, listen: false);
    var pageSource = Provider.of<PagesSource?>(context, listen: false);
    var uid = downSource?.uid ?? pageSource?.uid ?? "";

    if (parsed.scheme != "" && parsed.scheme == "br") {
      if (!await launchUrl(Uri.parse(url))) {
        showErrorSnackbar(context, "Could not launch $url");
      }
    }

    // Handle absolute br:// link.
    if (parsed.host != "") {
      uid = parsed.host;
    }

    if (uid == "") {
      throw "Cannot follow br:// link without target UID";
    }

    var resources = Provider.of<ResourcesModel>(context, listen: false);
    var sessionID = pageSource?.sessionID ?? 0;
    var parentPageID = pageSource?.pageID ?? 0;
    try {
      await resources.fetchPage(
          uid, parsed.pathSegments, sessionID, parentPageID, null);
    } catch (exception) {
      showErrorSnackbar(context, "Unable to fetch page: $exception");
    }
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var darkTextColor = theme.indicatorColor;
    var textColor = theme.focusColor;
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => MarkdownBody(
              codeBlockMaxHeight: 200,
              styleSheet: MarkdownStyleSheet(
                p: TextStyle(
                    fontSize: 8 + theme.getFontSize() * 7,
                    color: textColor,
                    fontWeight: hasNick ? FontWeight.w700 : FontWeight.w300,
                    letterSpacing: 0.44),
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
                codeblockDecoration: BoxDecoration(
                    color: Colors.black87,
                    borderRadius: BorderRadius.circular(2),
                    border: Border.all(width: 0.5, color: Colors.white24)),
                code: TextStyle(
                    fontFeatures: const [FontFeature.tabularFigures()],
                    color: textColor,
                    backgroundColor: Colors.black87,
                    height: 1.2,
                    fontSize: 8 + theme.getFontSize() * 7,
                    fontFamily: 'Inter',
                    textBaseline: TextBaseline.alphabetic),
              ),
              data: text.trim(),
              extensionSet: md.ExtensionSet(
                  md.ExtensionSet.gitHubFlavored.blockSyntaxes, [
                md.EmojiSyntax(),
                ...md.ExtensionSet.gitHubFlavored.inlineSyntaxes
              ]),
              builders: {
                //"video": VideoMarkdownElementBuilder(basedir),
                "codeblock": CodeblockMarkdownElementBuilder(),
                "image": ImageMarkdownElementBuilder(),
                "download":
                    DownloadLinkElementBuilder(TextStyle(color: darkTextColor)),
                "form": _FormElementBuilder(
                    labelStyle: TextStyle(color: textColor)),
              },
              onTapLink: (text, url, blah) {
                launchUrlAwait(context, url);
              },
              inlineSyntaxes: [
                //VideoInlineSyntax(),
                //ImageInlineSyntax()
                EmbedInlineSyntax(),
              ],
              blockSyntaxes: [
                _FormBlockSyntax(),
              ],
            ));
  }
}

class Downloadable extends StatelessWidget {
  final String tip;
  final String fid;
  final Widget child;
  const Downloadable(this.tip, this.fid, this.child, {Key? key})
      : super(key: key);

  void download(BuildContext context) async {
    try {
      var downloads = Provider.of<DownloadsModel>(context, listen: false);
      var source = Provider.of<DownloadSource?>(context, listen: false);
      var page = Provider.of<PagesSource?>(context, listen: false);
      var uid = source?.uid ?? page?.uid ?? "";
      if (uid == "") {
        throw "UID in parent DownloadsSource/PagesSource not found";
      }
      await downloads.getUnknownUserFile(uid, fid);
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
          ),
        ),
      );
}

class ImageMd extends StatelessWidget {
  final String tip;
  final Uint8List imgContent;
  const ImageMd(this.tip, this.imgContent, {Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) => Tooltip(
        message: tip,
        child: InkWell(
          borderRadius: const BorderRadius.all(Radius.circular(30)),
          hoverColor: Theme.of(context).hoverColor,
          onTap: () {
            showDialog(
                context: context, builder: (_) => ImageDialog(imgContent));
          },
          child: Container(
            constraints: const BoxConstraints(maxHeight: 250, maxWidth: 250),
            margin: const EdgeInsets.symmetric(horizontal: 2, vertical: 2),
            decoration: BoxDecoration(
              borderRadius: const BorderRadius.all(Radius.circular(8.0)),
              image: DecorationImage(
                image: MemoryImage(imgContent),
              ),
            ),
          ),
        ),
      );
}

class CodeblockMarkdownElementBuilder extends MarkdownElementBuilder {
  @override
  Widget visitText(md.Text text, TextStyle? preferredStyle) {
    var style = preferredStyle!.copyWith(
      backgroundColor: Colors.transparent,
    );
    return Text.rich(
      TextSpan(text: text.text),
      style: style,
    );
  }
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

    try {
      return ImageMd(tip, imgBytes);
    } catch (exception) {
      print("Unable to decode image: $exception");
      return Image.asset(
        "assets/images/invalidimg.png",
        width: 300,
        height: 300,
        fit: BoxFit.cover,
      );
    }
  }
}

class _FormSubmitButton extends StatelessWidget {
  final _FormElement form;
  final _FormField submit;
  const _FormSubmitButton(this.form, this.submit, {super.key});

  void doSubmit(BuildContext context, _FormElement form) async {
    Map<String, dynamic> formData = {};
    String action = "";
    for (var field in form.fields) {
      if (field.type == "action") {
        action = field.value ?? "";
      }
      if (field.name == "" || field.value == null) {
        continue;
      }
      formData[field.name] = field.value;
    }

    if (action == "") {
      return;
    }

    var parsed = Uri.parse(action);

    var downSource = Provider.of<DownloadSource?>(context, listen: false);
    var pageSource = Provider.of<PagesSource?>(context, listen: false);
    var uid = downSource?.uid ?? pageSource?.uid ?? "";

    var resources = Provider.of<ResourcesModel>(context, listen: false);
    var sessionID = pageSource?.sessionID ?? 0;
    var parentPageID = pageSource?.pageID ?? 0;

    try {
      await resources.fetchPage(
          uid, parsed.pathSegments, sessionID, parentPageID, formData);
    } catch (exception) {
      showErrorSnackbar(context, "Unable to fetch page: $exception");
    }
  }

  @override
  Widget build(BuildContext context) {
    return ElevatedButton(
        onPressed: () => doSubmit(context, form), child: Text(submit.label));
  }
}

class _FormElementBuilder extends MarkdownElementBuilder {
  final TextStyle? labelStyle;
  _FormElementBuilder({this.labelStyle = null});

  @override
  Widget visitElementAfter(md.Element element, TextStyle? preferredStyle) {
    if (element is! _FormElement) {
      return const Text("not-a-form-element",
          style: TextStyle(color: Colors.amber));
    }

    _FormElement form = element;
    _FormField? submit;

    List<Tuple2<Widget, Widget>> fieldWidgets = [];
    form.fields.forEach((field) {
      switch (field.type) {
        case "txtinput":
          TextEditingController ctrl = TextEditingController();
          if (field.value is String) {
            ctrl.value = field.value;
          }
          fieldWidgets.add(Tuple2(Text(field.label, style: labelStyle),
              TextField(onChanged: (String val) {
            field.value = val;
          })));
          break;
        case "intinput":
          IntEditingController ctrl = IntEditingController();
          if (field.value is int) {
            ctrl.intvalue = field.value;
          } else if (field.value is double) {
            ctrl.intvalue = (field.value as double).truncate();
          } else if (field.value is String) {
            ctrl.intvalue = int.tryParse(field.value as String) ?? 0;
          }
          field.value = ctrl.intvalue;
          fieldWidgets.add(Tuple2(
              Text(field.label, style: labelStyle),
              intInput(
                  controller: ctrl,
                  onChanged: (int val) {
                    field.value = val;
                  })));
          break;
        case "submit":
          submit = field;
          break;
        case "hidden":
        case "action":
          break;
        default:
          print("Unknown field type ${field.type}");
      }
    });

    return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
      SimpleInfoGrid(fieldWidgets),
      const SizedBox(height: 10),
      submit != null ? _FormSubmitButton(form, submit!) : const Empty(),
    ]);
  }
}
