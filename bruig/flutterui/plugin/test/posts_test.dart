import 'package:flutter_test/flutter_test.dart';
import 'package:golib_plugin/definitions.dart';

void main() {
  test("post metadata status hashes correctly", () {
    expect(PostMetadataStatus(0, "", "", {}).hash(),
        "05f6ac47accd338d329cc16f6d59f3409cc8bfe76a272e1eec612e49c115145d");
    expect(PostMetadataStatus(1, "", "", {}).hash(),
        "8ea40918f0472ddddd8ee06fabfebcdeca0cad2fe4a069c7e4172b819c2ee507");
    expect(PostMetadataStatus(0xac5be174813d6559, "", "", {}).hash(),
        "77c4e7a2c54f60f73b742f2cc654dc3e1770a863b76cfd9b10dfbb06179dd449");
    expect(PostMetadataStatus(1, "0001020304", "", {}).hash(),
        "b03bf75296affbbaaeeba07bacf239060737406373d237707e0ea8f415d3d077");
    expect(
        PostMetadataStatus(1, "", "", {RMPIdentifier: "000102030405"}).hash(),
        "1449ad9e411f8bb5c7680bd7c3721d04aff728cc48c7261b6211cb20d237500f");
    expect(
        PostMetadataStatus(1, "", "", {RMPSComment: "comment ウェブの国際化"}).hash(),
        "b6e4860f17c2be85c95a0d2fbd1175febbb32dbbd298ec6df25ead012d82b61e");
    expect(
        PostMetadataStatus(1, "", "", {
          RMPIdentifier: "000102030405",
          RMPSComment: "comment ウェブの国際化"
        }).hash(),
        "e280e0cd9347f3ec8a29e1b9e80c634d92ca9eb6557eac88651edcf183e7a45d");
  });
}
