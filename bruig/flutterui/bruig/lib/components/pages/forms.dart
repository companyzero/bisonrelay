import 'package:bruig/components/empty_widget.dart';
import 'package:bruig/components/md_elements.dart';
import 'package:bruig/components/inputs.dart';
import 'package:bruig/models/resources.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:flutter/material.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:markdown/markdown.dart' as md;
import 'package:provider/provider.dart';
import 'package:flutter/services.dart';

class _FormSubmitButton extends StatelessWidget {
  final FormElement form;
  final FormField submit;
  final GlobalKey<FormState> formKey;
  const _FormSubmitButton(this.form, this.submit, this.formKey);

  void doSubmit(BuildContext context, FormElement form) async {
    var snackbar = SnackBarModel.of(context);
    Map<String, dynamic> formData = {};
    String action = "";
    String asyncTargetID = "";
    for (var field in form.fields) {
      if (field.type == "action") {
        action = field.value ?? "";
      }
      if (field.type == "asynctarget") {
        asyncTargetID = field.value ?? "";
        continue;
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
      await resources.fetchPage(uid, parsed.pathSegments, sessionID,
          parentPageID, formData, asyncTargetID);
    } catch (exception) {
      snackbar.error("Unable to fetch page: $exception");
    }
  }

  @override
  Widget build(BuildContext context) {
    return ElevatedButton(
        onPressed: () {
          if (formKey.currentState!.validate()) {
            doSubmit(context, form);
          }
        },
        child: Text(submit.label));
  }
}

class FormElementBuilder extends MarkdownElementBuilder {
  FormElementBuilder();

  @override
  Widget visitElementAfter(md.Element element, TextStyle? preferredStyle) {
    if (element is! FormElement) {
      return const Text("not-a-form-element",
          style: TextStyle(color: Colors.amber));
    }

    FormElement form = element;
    return CustomForm(form);
  }
}

class CustomForm extends StatefulWidget {
  final FormElement form;
  const CustomForm(this.form, {super.key});

  @override
  CustomFormState createState() {
    return CustomFormState();
  }
}

class CustomFormState extends State<CustomForm> {
  final _formKey = GlobalKey<FormState>();
  FormElement get form => widget.form;
  @override
  Widget build(BuildContext context) {
    FormField? submit;

    List<Widget> fieldWidgets = [];
    for (var field in form.fields) {
      switch (field.type) {
        case "txtinput":
          TextEditingController ctrl = TextEditingController();
          if (field.value is String) {
            ctrl.text = field.value;
          }
          fieldWidgets.add(TextFormField(
              controller: ctrl,
              decoration: InputDecoration(
                hintText: field.hint,
                labelText: field.label,
              ),
              onSaved: (String? value) {
                // This optional block of code can be used to run
                // code when the user saves the form.
              },
              validator: (String? value) {
                if (value != null && field.regexp != "") {
                  return RegExp(field.regexp).hasMatch(value)
                      ? null
                      : field.regexpstr;
                }
                return null;
              },
              onChanged: (String val) {
                field.value = val;
                _formKey.currentState!.validate();
              }));

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
          fieldWidgets.add(TextFormField(
            controller: ctrl,
            keyboardType: TextInputType.number,
            inputFormatters: <TextInputFormatter>[
              FilteringTextInputFormatter.digitsOnly
            ],
            decoration: InputDecoration(
              labelText: field.label,
              hintText: field.hint,
            ),
            onChanged: (String val) {
              field.value = val;
            },
            validator: (String? value) {
              if (value != null && field.regexp != "") {
                return RegExp(field.regexp).hasMatch(value)
                    ? null
                    : field.regexpstr;
              }
              return null;
            },
          ));
          break;
        case "submit":
          submit = field;
          break;
        case "asynctarget":
        case "hidden":
        case "action":
          break;
        default:
          debugPrint("Unknown field type ${field.type}");
      }
    }

    // Build a Form widget using the _formKey created above.
    return Form(
      key: _formKey,
      child: Column(
        children: <Widget>[
          ...fieldWidgets,
          const SizedBox(height: 10),
          submit != null
              ? _FormSubmitButton(form, submit, _formKey)
              : const Empty(),
          // Add TextFormFields and ElevatedButton here.
        ],
      ),
    );
  }
}

class FormField {
  final String type;
  final String name;
  final String label;
  dynamic value;
  final String regexp;
  final String regexpstr;
  final String hint;

  FormField(this.type,
      {this.name = "",
      this.label = "",
      this.regexp = "",
      this.regexpstr = "",
      this.hint = "",
      this.value});
}

class FormElement extends md.Element {
  final List<FormField> fields;

  FormElement(this.fields) : super("form", [md.Text("")]);
}

class FormBlockSyntax extends md.BlockSyntax {
  static String closeTag = r'--/form--';
  static RegExp tagPattern = RegExp(r'^--form--$');
  static RegExp fieldPattern = RegExp(r'([\w]+)="([^"]*)"');

  @override
  RegExp get pattern => tagPattern;

  @override
  bool canEndBlock(md.BlockParser parser) =>
      parser.current.content == "--/form--";

  @override
  md.Node? parse(md.BlockParser parser) {
    parser.advance();
    List<FormField> children = [];

    while (!parser.isDone && !md.BlockSyntax.isAtBlockEnd(parser)) {
      if (parser.current.content == closeTag) {
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
          case "regexp":
          case "regexpstr":
            args[Symbol(name)] = value;
            break;
        }
      }

      FormField field = Function.apply(FormField.new, [type], args);
      children.add(field);
      parser.advance();
    }

    var res = md.Element("p", [FormElement(children)]);
    return res;
  }
}
