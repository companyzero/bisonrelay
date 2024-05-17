import 'package:bruig/components/snackbars.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';

typedef OnAccountChanged = void Function(String);

class AccountsDropDown extends StatefulWidget {
  final OnAccountChanged? onChanged;
  final bool excludeDefault;
  const AccountsDropDown(
      {this.onChanged, this.excludeDefault = false, super.key});

  @override
  State<AccountsDropDown> createState() => _AccountsDropDownState();
}

class _AccountsDropDownState extends State<AccountsDropDown> {
  String? selected;
  List<Account> accounts = [];

  void reloadAccounts() async {
    try {
      var newAccounts = await Golib.listAccounts();
      if (widget.excludeDefault) {
        newAccounts.removeAt(0);
      }
      setState(() {
        accounts = newAccounts;
        if (accounts.isNotEmpty && selected == null) {
          selected = accounts[0].name;
          if (widget.onChanged != null) {
            widget.onChanged!(selected!);
          }
        }
      });
    } catch (exception) {
      showErrorSnackbar(context, "Unable to load accounts: $exception");
    }
  }

  @override
  void initState() {
    super.initState();
    reloadAccounts();
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    var backgroundColor = theme.backgroundColor;
    var tsSelected = TextStyle(color: backgroundColor);
    var tsRegular = TextStyle(color: textColor);
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => DropdownButton<String?>(
              isDense: true,
              isExpanded: true,
              icon: Icon(
                Icons.arrow_downward,
                color: textColor,
              ),
              underline: Container(),
              value: selected,
              padding: EdgeInsets.zero,
              style: TextStyle(
                  color: textColor, fontSize: theme.getSmallFont(context)),
              selectedItemBuilder: (context) {
                return accounts
                    // This is backwards than what you'd expect (tsRegular vs tsSelected)
                    // because otherwise it doesn't look correct. Maybe a misuse or
                    // bug in DropDownMenu.
                    .map((e) => Text(e.name, style: tsRegular))
                    .toList();
              },
              items: (accounts
                  .map<DropdownMenuItem<String?>>((e) => DropdownMenuItem(
                        value: e.name,
                        child: Text(e.name,
                            style: e.name == selected ? tsSelected : tsRegular),
                      ))).toList(),
              onChanged: (newValue) {
                var didChange = newValue != selected;
                setState(() {
                  selected = newValue;
                });
                if (didChange && widget.onChanged != null) {
                  widget.onChanged!(newValue!);
                }
              },
            ));
  }
}
