import 'package:bruig/components/snackbars.dart';
import 'package:bruig/models/client.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:provider/provider.dart';

class NewGCScreen extends StatefulWidget {
  const NewGCScreen({Key? key}) : super(key: key);

  @override
  NewGCScreenState createState() {
    return NewGCScreenState();
  }
}

class NewGCScreenState extends State<NewGCScreen> {
  final _formKey = GlobalKey<FormState>();
  bool loading = false;
  String gcName = "";

  void createGCPressed(ClientModel client) async {
    if (loading) return;
    if (!_formKey.currentState!.validate()) return;
    _formKey.currentState!.save();
    if (gcName == "") return;

    setState(() => loading = true);
    try {
      await Golib.createGC(gcName);
      await client.readAddressBook();
      Navigator.pop(context);
    } catch (exception) {
      showErrorSnackbar(context, 'Unable to create GC: $exception');
    } finally {
      setState(() => loading = false);
    }
  }

  void onDone(BuildContext context) {
    Navigator.pop(context);
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    return Scaffold(
      body: Center(
        child: Container(
            padding: const EdgeInsets.all(40),
            constraints: const BoxConstraints(maxWidth: 500),
            child: Form(
                key: _formKey,
                child: Column(children: [
                  Wrap(
                    runSpacing: 10,
                    children: <Widget>[
                      TextFormField(
                          decoration: const InputDecoration(
                            icon: Icon(Icons.chat),
                            labelText: 'GC Name',
                          ),
                          onSaved: (String? v) => gcName = v?.trim() ?? "",
                          validator: (String? value) =>
                              value != null && value.trim().isEmpty
                                  ? 'Cannot be blank'
                                  : null),
                      Container(height: 20),
                      Consumer<ClientModel>(
                          builder: (context, client, child) => ElevatedButton(
                              onPressed: !loading
                                  ? () => createGCPressed(client)
                                  : null,
                              child: const Text('Create GC'))),
                      Container(height: 10),
                      ElevatedButton(
                          style: ElevatedButton.styleFrom(
                              backgroundColor: theme.errorColor),
                          onPressed: !loading ? () => onDone(context) : null,
                          child: const Text("Cancel"))
                    ],
                  )
                ]))),
      ),
    );
  }
}
