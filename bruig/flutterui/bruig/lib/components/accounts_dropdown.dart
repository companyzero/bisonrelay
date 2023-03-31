import 'package:bruig/components/snackbars.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';

typedef void OnAccountChanged(String);

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
            widget.onChanged!(selected);
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
    return DropdownButton<String?>(
      focusColor: Colors.red,
      isDense: true,
      isExpanded: true,
      icon: Icon(
        Icons.arrow_downward,
        color: textColor,
      ),
      dropdownColor: backgroundColor,
      underline: Container(),
      value: selected,
      items: (accounts.map<DropdownMenuItem<String?>>((e) => DropdownMenuItem(
            value: e.name,
            child: Container(
                margin: const EdgeInsets.all(0),
                width: double.infinity,
                alignment: Alignment.centerLeft,
                child: Text(e.name,
                    style: TextStyle(
                      color: textColor,
                      fontSize: 11,
                    ))),
          ))).toList(),
      onChanged: (newValue) {
        var didChange = newValue != selected;
        setState(() {
          selected = newValue;
        });
        if (didChange && widget.onChanged != null) {
          widget.onChanged!(newValue);
        }
      },
    );
  }
}
