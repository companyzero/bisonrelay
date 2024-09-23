import 'dart:convert';
import 'dart:io';
// import 'package:dart_vlc/dart_vlc.dart' as vlc;
import 'package:bruig/components/context_menu.dart';
import 'package:bruig/components/pages/forms.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/components/text_dialog.dart';
import 'package:bruig/models/downloads.dart';
import 'package:bruig/models/payments.dart';
import 'package:bruig/models/resources.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:golib_plugin/util.dart';
import 'package:markdown/markdown.dart' as md;
import 'package:path_provider/path_provider.dart';
import 'package:provider/provider.dart';
import 'package:share_plus/share_plus.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:bruig/theme_manager.dart';
import 'package:bruig/components/image_dialog.dart';
import 'package:pdfrx/pdfrx.dart';
import 'package:path/path.dart' as path;

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

class LnpayURLSyntax extends md.InlineSyntax {
  LnpayURLSyntax({
    String pattern = r'lnpay:\/\/(ln[td]?\w*)',
  }) : super(pattern);

  @override
  bool onMatch(md.InlineParser parser, Match match) {
    final url = match.group(1) ?? "";

    md.Element el = md.Element.text("lnpay", url);

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
      case "image/avif":
      case "image/bmp":
      case "image/gif":
      case "image/jpeg":
      case "image/jxl":
      case "image/png":
      case "image/webp":
        tag = "image";
        break;
      case "text/plain":
        // Decode plain text directly.
        tag = "pre";
        try {
          data = utf8.fuse(base64).decode(data);
        } catch (exception) {
          data = "Unable to decode plain text contents: $exception";
        }
        break;
      case "application/pdf":
        tag = "pdf";
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

    if (parms["type"] != "") {
      el.attributes["type"] = parms["type"]!;
    }

    if (parms.containsKey("filename") && parms["filename"] != "") {
      el.attributes["filename"] = parms["filename"]!;
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
  static final extensionSet = md.ExtensionSet(
      md.ExtensionSet.gitHubFlavored.blockSyntaxes,
      [md.EmojiSyntax(), ...md.ExtensionSet.gitHubFlavored.inlineSyntaxes]);

  static final builders = {
    "pre": PreformattedElementBuilder(),
    "pdf": PDFMarkdownElementBuilder(),
    //"video": VideoMarkdownElementBuilder(basedir),
    "codeblock": CodeblockMarkdownElementBuilder(),
    "image": ImageMarkdownElementBuilder(),
    "download": DownloadLinkElementBuilder(),
    "form": FormElementBuilder(),
    "lnpay": _LNPayURLElementBuilder(),
  };

  static final inlineSyntaxes = [
    //VideoInlineSyntax(),
    //ImageInlineSyntax()
    EmbedInlineSyntax(),
    LnpayURLSyntax(),
  ];

  static final blockSyntaxes = [
    FormBlockSyntax(),
  ];

  static final _startTagBugRe = RegExp(r'^\s*(<[^>\s]+\s*>)$');

  static String _cleanupSrcText(String text) {
    // This renderer has a bug where a raw text "<foo>" needs escaping, otherwise
    // its not rendered.
    return text.replaceFirstMapped(_startTagBugRe, (m) => "\\${m[1]}");
  }

  final String text;
  final bool hasNick;
  MarkdownArea(srcText, this.hasNick, {Key? key})
      : text = MarkdownArea._cleanupSrcText(srcText),
        super(key: key);

  Future<void> launchUrlAwait(context, url) async {
    var parsed = Uri.parse(url);
    var downSource = Provider.of<DownloadSource?>(context, listen: false);
    var pageSource = Provider.of<PagesSource?>(context, listen: false);
    var uid = downSource?.uid ?? pageSource?.uid ?? "";
    var snackbar = SnackBarModel.of(context);

    if (parsed.scheme != "" && parsed.scheme != "br") {
      if (!await launchUrl(Uri.parse(url))) {
        snackbar.error("Could not launch $url");
      }
      return;
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
      snackbar.error("Unable to fetch page: $exception");
    }
  }

  @override
  Widget build(BuildContext context) {
    return Consumer2<ThemeNotifier, PaymentsModel>(
        builder: (context, theme, payments, _) => MarkdownBody(
              codeBlockMaxHeight: 200,
              styleSheet: theme.mdStyleSheet,
              data: text.trim(),
              extensionSet: extensionSet,
              builders: builders,
              onTapLink: (text, url, _) {
                launchUrlAwait(context, url);
              },
              inlineSyntaxes: inlineSyntaxes,
              blockSyntaxes: blockSyntaxes,
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
    var snackbar = SnackBarModel.of(context);
    try {
      var downloads = Provider.of<DownloadsModel>(context, listen: false);
      var source = Provider.of<DownloadSource?>(context, listen: false);
      var page = Provider.of<PagesSource?>(context, listen: false);
      var uid = source?.uid ?? page?.uid ?? "";
      if (uid == "") {
        throw "UID in parent DownloadsSource/PagesSource not found";
      }
      await downloads.getUnknownUserFile(uid, fid);
      snackbar.success("Added $fid to download queue");
    } catch (exception) {
      snackbar.error("Unable to start download: $exception");
    }
  }

  @override
  Widget build(BuildContext context) => Tooltip(
        message: tip,
        child: InkWell(
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
  final String type;
  const ImageMd(this.tip, this.imgContent, this.type, {Key? key})
      : super(key: key);

  @override
  Widget build(BuildContext context) => Tooltip(
        message: tip,
        child: InkWell(
          borderRadius: const BorderRadius.all(Radius.circular(30)),
          onTap: () {
            showDialog(
                context: context,
                builder: (_) => ImageDialog(imgContent, type));
          },
          child: Container(
            constraints: const BoxConstraints(maxHeight: 250, maxWidth: 250),
            margin: const EdgeInsets.symmetric(horizontal: 2, vertical: 2),
            decoration: BoxDecoration(
              borderRadius: const BorderRadius.all(Radius.circular(8.0)),
              image: DecorationImage(
                image: MemoryImage(imgContent),
                onError: (exception, stackTrace) {
                  debugPrint("ImageMd unable to decode image: $exception");
                },
              ),
            ),
          ),
        ),
      );
}

class PreformattedElementBuilder extends MarkdownElementBuilder {
  @override
  Widget visitText(md.Text text, TextStyle? preferredStyle) {
    return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
      ConstrainedBox(
        constraints: const BoxConstraints(maxHeight: 200),
        child: SingleChildScrollView(
            controller: ScrollController(keepScrollOffset: false),
            child: Consumer<ThemeNotifier>(
                builder: (context, theme, child) => Text.rich(
                      TextSpan(text: text.text),

                      // Overwrite <pre> style to use the same as code
                      // (Markdown component uses same as <p> by default).
                      style: theme.mdStyleSheet.code,
                    ))),
      ),
      const SizedBox(height: 10),
      Builder(
          builder: (context) => TextButton(
              onPressed: () => showDialog(
                  context: context,
                  builder: (context) => TextDialog(text.text)),
              child: const Text("View"))),
    ]);
  }
}

class CodeblockMarkdownElementBuilder extends MarkdownElementBuilder {
  @override
  Widget visitText(md.Text text, TextStyle? preferredStyle) {
    return Text.rich(
      TextSpan(text: text.text),
      style: preferredStyle,
    );
  }
}

class PDFMarkdownElementBuilder extends MarkdownElementBuilder {
  Future<String> _tempPDFDir() async {
    bool isMobile = Platform.isIOS || Platform.isAndroid;
    String base = isMobile
        ? (await getApplicationCacheDirectory()).path
        : (await getDownloadsDirectory())?.path ?? "";
    return path.join(base, "feedimages");
  }

  void _handleItemTap(BuildContext context, String value, Uint8List pdfBytes,
      String filename) async {
    switch (value) {
      case "save":
        var fname = await FilePicker.platform.saveFile(
              dialogTitle: "Select filename",
              fileName: filename != "" ? filename : "document.pdf",
            ) ??
            "";

        if (fname == "") {
          return;
        }

        File(fname).writeAsBytesSync(pdfBytes);
        context.mounted
            ? showSuccessSnackbar(context, "Written PDF file $fname")
            : null;
        break;

      case "share":
        var fname = filename != "" ? filename : "document.pdf";
        var dir = await _tempPDFDir();
        if (!Directory(dir).existsSync()) {
          Directory(dir).createSync(recursive: true);
        }
        fname = path.join(dir, fname);
        File(fname).writeAsBytesSync(pdfBytes);
        Share.shareXFiles([XFile(fname)], text: "Pdf");
        break;
    }
  }

  @override
  Widget visitElementAfter(md.Element element, TextStyle? preferredStyle) {
    Uint8List pdfBytes;
    String filename = element.attributes["filename"] ?? "";
    try {
      pdfBytes = const Base64Decoder().convert(element.textContent);
      if (pdfBytes.isEmpty) throw "Empty PDF";
    } catch (exception) {
      return Text("Unable to decode pdf: $exception");
    }

    try {
      return Builder(
          builder: (context) => ContextMenu(
              handleItemTap: (value) {
                _handleItemTap(context, value, pdfBytes, filename);
              },
              items: [
                if (!Platform.isAndroid)
                  const PopupMenuItem(
                      value: "save", child: Text("Save to file")),
                if (Platform.isAndroid || Platform.isIOS)
                  const PopupMenuItem(value: "share", child: Text("Share")),
              ],
              child: ConstrainedBox(
                  constraints:
                      const BoxConstraints(maxWidth: 400, maxHeight: 400),
                  child: PdfViewer(
                    PdfDocumentRefData(pdfBytes, sourceName: "data"),
                  ))));
    } catch (exception) {
      debugPrint("Unable to decode pdf: $exception");
      return Image.asset(
        "assets/images/invalidimg.png",
        width: 300,
        height: 300,
        fit: BoxFit.cover,
      );
    }
  }
}

class DownloadLinkElementBuilder extends MarkdownElementBuilder {
  DownloadLinkElementBuilder();

  @override
  Widget visitElementAfter(md.Element element, TextStyle? preferredStyle) {
    var download = element.attributes["fid"] ?? "";
    var tip = "Click to download file $download";
    return Downloadable(tip, download, Text(element.textContent));
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
    var type = element.attributes["type"] ?? "";

    try {
      return ImageMd(tip, imgBytes, type);
    } catch (exception) {
      debugPrint("Unable to decode image: $exception");
      return Image.asset(
        "assets/images/invalidimg.png",
        width: 300,
        height: 300,
        fit: BoxFit.cover,
      );
    }
  }
}

class _PayReqBtn extends StatefulWidget {
  final PaymentsModel payments;
  final String invoice;
  const _PayReqBtn(this.payments, this.invoice);

  @override
  State<_PayReqBtn> createState() => __PayReqBtnState();
}

class __PayReqBtnState extends State<_PayReqBtn> {
  late PaymentInfo info;

  void payInfoChanged() {
    setState(() {});
  }

  void attemptPayment() {
    info.attemptPayment();
  }

  @override
  void initState() {
    super.initState();
    info = widget.payments.decodedInvoice(widget.invoice);
    info.addListener(payInfoChanged);
  }

  @override
  void dispose() {
    info.removeListener(payInfoChanged);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (info.decoded == null) {
      return const ElevatedButton(
          onPressed: null, child: Text("Decoding invoice..."));
    }

    String amt = formatDCR(info.decoded?.amount ?? 0);

    if (info.status == PaymentStatus.succeeded) {
      return ElevatedButton(
          onPressed: null, child: Text("Succeeded paying $amt"));
    }

    if (info.status == PaymentStatus.errored) {
      return ElevatedButton(
          onPressed: null, child: Text("Errored paying $amt: ${info.err}"));
    }

    if (info.status == PaymentStatus.inflight) {
      return ElevatedButton(onPressed: null, child: Text("Paying $amt"));
    }

    if (info.decoded?.expired ?? false) {
      return ElevatedButton(
          onPressed: null, child: Text("Invoice $amt expired"));
    }

    return ElevatedButton(onPressed: attemptPayment, child: Text("Pay $amt"));
  }
}

class _LNPayURLElementBuilder extends MarkdownElementBuilder {
  @override
  Widget visitElementAfter(md.Element element, TextStyle? preferredStyle) {
    return Consumer<PaymentsModel>(
        builder: (context, payments, child) =>
            _PayReqBtn(payments, element.textContent));
  }
}
