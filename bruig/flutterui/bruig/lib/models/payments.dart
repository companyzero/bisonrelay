import 'package:flutter/foundation.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';

enum PaymentStatus {
  unknown,
  inflight,
  errored,
  succeeded,
}

class PaymentInfo extends ChangeNotifier {
  final String invoice;

  LNDecodedInvoice? _decoded;

  PaymentInfo(this.invoice);
  LNDecodedInvoice? get decoded => _decoded;
  Future<void> decode() async {
    try {
      _decoded = await Golib.lnDecodeInvoice(invoice);
      notifyListeners();
    } catch (exception) {
      print("Unable to decode invoice: $exception");
    }
  }

  Exception? _err;
  Exception? get err => _err;

  PaymentStatus _status = PaymentStatus.unknown;
  PaymentStatus get status => _status;
  void attemptPayment() async {
    if (status != PaymentStatus.unknown) {
      // Already attempting payment.
      return;
    }

    _status = PaymentStatus.inflight;
    notifyListeners();
    try {
      await Golib.lnPayInvoice(invoice, decoded?.amount ?? 0);
      _status = PaymentStatus.succeeded;
    } catch (exception) {
      _err = Exception(exception);
      _status = PaymentStatus.errored;
    }
    notifyListeners();
  }
}

class PaymentsModel extends ChangeNotifier {
  final Map<String, PaymentInfo> byInvoice = {};

  PaymentInfo decodedInvoice(String invoice) {
    if (byInvoice.containsKey(invoice)) {
      return byInvoice[invoice]!;
    }

    var pay = PaymentInfo(invoice);
    byInvoice[invoice] = pay;
    pay.decode();
    return pay;
  }
}
