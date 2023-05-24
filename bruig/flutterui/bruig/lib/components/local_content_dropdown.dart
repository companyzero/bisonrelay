import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';

typedef LocalContentDropDownChanged = void Function(SharedFileAndShares?);

class LocalContentDropDown extends StatefulWidget {
  final bool allowEmpty;
  final LocalContentDropDownChanged? onChanged;
  const LocalContentDropDown(this.allowEmpty, {this.onChanged, super.key});

  @override
  State<LocalContentDropDown> createState() => _LocalContentDropDownState();
}

class _LocalContentDropDownState extends State<LocalContentDropDown> {
  List<SharedFileAndShares?> files = [];
  SharedFileAndShares? selected;

  Future<void> loadSharedContent() async {
    var newfiles = await Golib.listSharedFiles();
    newfiles.sort((SharedFileAndShares a, SharedFileAndShares b) {
      // Sort by dir, then filename.
      return a.sf.filename.compareTo(b.sf.filename);
    });
    setState(() {
      files =
          newfiles.map<SharedFileAndShares?>((e) => e).toList(growable: true);
      if (widget.allowEmpty) {
        files.insert(0, null);
      }
    });
  }

  @override
  void initState() {
    super.initState();
    loadSharedContent();
  }

  String fileLabel(SharedFileAndShares? f) {
    if (f == null) {
      return "";
    }
    return f.sf.filename;
  }

  void changed(SharedFileAndShares? newValue) {
    setState(() {
      selected = newValue;
    });
    if (widget.onChanged != null) {
      widget.onChanged!(newValue);
    }
  }

  @override
  Widget build(BuildContext context) {
    return DropdownButton<SharedFileAndShares?>(
        value: selected,
        items: files
            .map((e) => DropdownMenuItem<SharedFileAndShares?>(
                value: e, child: Text(fileLabel(e))))
            .toList(),
        onChanged: changed);
  }
}
