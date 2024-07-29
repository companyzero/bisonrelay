// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'definitions.dart';

// **************************************************************************
// JsonSerializableGenerator
// **************************************************************************

InitClient _$InitClientFromJson(Map<String, dynamic> json) => InitClient(
      json['dbroot'] as String,
      json['downloads_dir'] as String,
      json['embeds_dir'] as String,
      json['server_addr'] as String,
      json['ln_rpc_host'] as String,
      json['ln_tls_cert_path'] as String,
      json['ln_macaroon_path'] as String,
      json['log_file'] as String,
      json['msgs_root'] as String,
      json['debug_level'] as String,
      json['wants_log_ntfns'] as bool,
      json['resources_upstream'] as String,
      json['simplestore_pay_type'] as String,
      json['simplestore_account'] as String,
      (json['simplestore_ship_charge'] as num).toDouble(),
      json['proxyaddr'] as String,
      json['torisolation'] as bool,
      json['proxy_username'] as String,
      json['proxy_password'] as String,
      json['circuit_limit'] as int,
      json['no_load_chat_history'] as bool,
      json['auto_handshake_interval'] as int,
      json['auto_remove_idle_users_interval'] as int,
      (json['auto_remove_idle_users_ignore'] as List<dynamic>)
          .map((e) => e as String)
          .toList(),
      json['send_recv_receipts'] as bool,
      json['auto_sub_posts'] as bool,
      json['log_pings'] as bool,
      json['ping_interval_ms'] as int,
    );

Map<String, dynamic> _$InitClientToJson(InitClient instance) =>
    <String, dynamic>{
      'dbroot': instance.dbRoot,
      'downloads_dir': instance.downloadsDir,
      'embeds_dir': instance.embedsDir,
      'server_addr': instance.serverAddr,
      'ln_rpc_host': instance.lnRPCHost,
      'ln_tls_cert_path': instance.lnTLSCertPath,
      'ln_macaroon_path': instance.lnMacaroonPath,
      'log_file': instance.logFile,
      'msgs_root': instance.msgsRoot,
      'debug_level': instance.debugLevel,
      'wants_log_ntfns': instance.wantsLogNtfns,
      'resources_upstream': instance.resourcesUpstream,
      'simplestore_pay_type': instance.simpleStorePayType,
      'simplestore_account': instance.simpleStoreAccount,
      'simplestore_ship_charge': instance.simpleStoreShipCharge,
      'proxyaddr': instance.proxyaddr,
      'proxy_username': instance.proxyUsername,
      'proxy_password': instance.proxyPassword,
      'torisolation': instance.torisolation,
      'circuit_limit': instance.circuitLimit,
      'no_load_chat_history': instance.noLoadChatHistory,
      'auto_handshake_interval': instance.autoHandshakeInterval,
      'auto_remove_idle_users_interval': instance.autoRemoveIdleUsersInterval,
      'auto_remove_idle_users_ignore': instance.autoRemoveIdleUsersIgnore,
      'send_recv_receipts': instance.sendRecvReceipts,
      'auto_sub_posts': instance.autoSubPosts,
      'log_pings': instance.logPings,
      'ping_interval_ms': instance.pingIntervalMs,
    };

IDInit _$IDInitFromJson(Map<String, dynamic> json) => IDInit(
      json['nick'] as String,
      json['name'] as String,
    );

Map<String, dynamic> _$IDInitToJson(IDInit instance) => <String, dynamic>{
      'nick': instance.nick,
      'name': instance.name,
    };

LocalInfo _$LocalInfoFromJson(Map<String, dynamic> json) => LocalInfo(
      json['id'] as String,
      json['nick'] as String,
    );

Map<String, dynamic> _$LocalInfoToJson(LocalInfo instance) => <String, dynamic>{
      'id': instance.id,
      'nick': instance.nick,
    };

ServerCert _$ServerCertFromJson(Map<String, dynamic> json) => ServerCert(
      json['inner_fingerprint'] as String,
      json['outer_fingerprint'] as String,
    );

Map<String, dynamic> _$ServerCertToJson(ServerCert instance) =>
    <String, dynamic>{
      'inner_fingerprint': instance.innerFingerprint,
      'outer_fingerprint': instance.outerFingerprint,
    };

ServerSessionState _$ServerSessionStateFromJson(Map<String, dynamic> json) =>
    ServerSessionState(
      json['state'] as int,
      json['check_wallet_err'] as String?,
    );

Map<String, dynamic> _$ServerSessionStateToJson(ServerSessionState instance) =>
    <String, dynamic>{
      'state': instance.state,
      'check_wallet_err': instance.checkWalletErr,
    };

ServerInfo _$ServerInfoFromJson(Map<String, dynamic> json) => ServerInfo(
      innerFingerprint: json['innerFingerprint'] as String,
      outerFingerprint: json['outerFingerprint'] as String,
      serverAddr: json['serverAddr'] as String,
    );

Map<String, dynamic> _$ServerInfoToJson(ServerInfo instance) =>
    <String, dynamic>{
      'innerFingerprint': instance.innerFingerprint,
      'outerFingerprint': instance.outerFingerprint,
      'serverAddr': instance.serverAddr,
    };

RemoteUser _$RemoteUserFromJson(Map<String, dynamic> json) => RemoteUser(
      json['uid'] as String,
      json['nick'] as String,
    );

Map<String, dynamic> _$RemoteUserToJson(RemoteUser instance) =>
    <String, dynamic>{
      'uid': instance.uid,
      'nick': instance.nick,
    };

InviteFunds _$InviteFundsFromJson(Map<String, dynamic> json) => InviteFunds(
      json['txid'] as String,
      json['index'] as int,
      json['tree'] as int,
      json['private_key'] as String,
      json['height_hint'] as int,
      json['address'] as String,
    );

Map<String, dynamic> _$InviteFundsToJson(InviteFunds instance) =>
    <String, dynamic>{
      'txid': instance.txid,
      'index': instance.index,
      'tree': instance.tree,
      'private_key': instance.privateKey,
      'height_hint': instance.heightHint,
      'address': instance.address,
    };

PublicIdentity _$PublicIdentityFromJson(Map<String, dynamic> json) =>
    PublicIdentity(
      json['name'] as String,
      json['nick'] as String,
      json['identity'] as String,
    );

Map<String, dynamic> _$PublicIdentityToJson(PublicIdentity instance) =>
    <String, dynamic>{
      'name': instance.name,
      'nick': instance.nick,
      'identity': instance.identity,
    };

OOBPublicIdentityInvite _$OOBPublicIdentityInviteFromJson(
        Map<String, dynamic> json) =>
    OOBPublicIdentityInvite(
      PublicIdentity.fromJson(json['public'] as Map<String, dynamic>),
      json['initialrendezvous'] as String,
      json['resetrendezvous'] as String,
      json['funds'] == null
          ? null
          : InviteFunds.fromJson(json['funds'] as Map<String, dynamic>),
    );

Map<String, dynamic> _$OOBPublicIdentityInviteToJson(
        OOBPublicIdentityInvite instance) =>
    <String, dynamic>{
      'public': instance.public,
      'initialrendezvous': instance.initialRendezvous,
      'resetrendezvous': instance.resetRendezvous,
      'funds': instance.funds,
    };

Invitation _$InvitationFromJson(Map<String, dynamic> json) => Invitation(
      OOBPublicIdentityInvite.fromJson(json['invite'] as Map<String, dynamic>),
      base64ToUint8list(json['blob'] as String?),
    );

Map<String, dynamic> _$InvitationToJson(Invitation instance) =>
    <String, dynamic>{
      'invite': instance.invite,
      'blob': instance.blob,
    };

GeneratedKXInvite _$GeneratedKXInviteFromJson(Map<String, dynamic> json) =>
    GeneratedKXInvite(
      base64Decode(json['blob'] as String),
      json['funds'] == null
          ? null
          : InviteFunds.fromJson(json['funds'] as Map<String, dynamic>),
      json['key'] as String,
    );

Map<String, dynamic> _$GeneratedKXInviteToJson(GeneratedKXInvite instance) =>
    <String, dynamic>{
      'blob': instance.blob,
      'funds': instance.funds,
      'key': instance.key,
    };

PM _$PMFromJson(Map<String, dynamic> json) => PM(
      json['sid'],
      json['msg'],
      json['mine'] as bool,
      json['timestamp'] as int,
    );

Map<String, dynamic> _$PMToJson(PM instance) => <String, dynamic>{
      'sid': instance.sid,
      'msg': instance.msg,
      'mine': instance.mine,
      'timestamp': instance.timestamp,
    };

InviteToGC _$InviteToGCFromJson(Map<String, dynamic> json) => InviteToGC(
      json['gc'] as String,
      json['uid'] as String,
    );

Map<String, dynamic> _$InviteToGCToJson(InviteToGC instance) =>
    <String, dynamic>{
      'gc': instance.gc,
      'uid': instance.uid,
    };

RMGroupInvite _$RMGroupInviteFromJson(Map<String, dynamic> json) =>
    RMGroupInvite(
      json['id'] as String,
      json['name'] as String,
      json['token'] as int,
      json['description'] as String,
      json['expires'] as int,
      json['version'] as int,
    );

Map<String, dynamic> _$RMGroupInviteToJson(RMGroupInvite instance) =>
    <String, dynamic>{
      'id': instance.id,
      'name': instance.name,
      'token': instance.token,
      'description': instance.description,
      'expires': instance.expires,
      'version': instance.version,
    };

GCAddressBookEntry _$GCAddressBookEntryFromJson(Map<String, dynamic> json) =>
    GCAddressBookEntry(
      json['id'] as String,
      json['name'] as String,
      (json['members'] as List<dynamic>).map((e) => e as String).toList(),
    );

Map<String, dynamic> _$GCAddressBookEntryToJson(GCAddressBookEntry instance) =>
    <String, dynamic>{
      'id': instance.id,
      'name': instance.name,
      'members': instance.members,
    };

RMGroupList _$RMGroupListFromJson(Map<String, dynamic> json) => RMGroupList(
      json['id'] as String,
      json['name'] as String,
      json['generation'] as int,
      json['timestamp'] as int,
      json['version'] as int,
      (json['members'] as List<dynamic>).map((e) => e as String).toList(),
      (json['extra_admins'] as List<dynamic>?)
          ?.map((e) => e as String)
          .toList(),
    );

Map<String, dynamic> _$RMGroupListToJson(RMGroupList instance) =>
    <String, dynamic>{
      'id': instance.id,
      'name': instance.name,
      'generation': instance.generation,
      'timestamp': instance.timestamp,
      'version': instance.version,
      'members': instance.members,
      'extra_admins': instance.extraAdmins,
    };

GCInvitation _$GCInvitationFromJson(Map<String, dynamic> json) => GCInvitation(
      RemoteUser.fromJson(json['inviter'] as Map<String, dynamic>),
      json['iid'] as int,
      json['name'] as String,
      RMGroupInvite.fromJson(json['invite'] as Map<String, dynamic>),
      json['accepted'] as bool,
    );

Map<String, dynamic> _$GCInvitationToJson(GCInvitation instance) =>
    <String, dynamic>{
      'inviter': instance.inviter,
      'iid': instance.iid,
      'name': instance.name,
      'invite': instance.invite,
      'accepted': instance.accepted,
    };

GCMsg _$GCMsgFromJson(Map<String, dynamic> json) => GCMsg(
      json['sender_uid'] as String,
      json['sid'],
      json['msg'],
      json['timestamp'] as int,
    );

Map<String, dynamic> _$GCMsgToJson(GCMsg instance) => <String, dynamic>{
      'sid': instance.sid,
      'msg': instance.msg,
      'sender_uid': instance.senderUID,
      'timestamp': instance.timestamp,
    };

GCMsgToSend _$GCMsgToSendFromJson(Map<String, dynamic> json) => GCMsgToSend(
      json['gc'] as String,
      json['msg'] as String,
    );

Map<String, dynamic> _$GCMsgToSendToJson(GCMsgToSend instance) =>
    <String, dynamic>{
      'gc': instance.gc,
      'msg': instance.msg,
    };

GCRemoveUserArgs _$GCRemoveUserArgsFromJson(Map<String, dynamic> json) =>
    GCRemoveUserArgs(
      json['gc'] as String,
      json['uid'] as String,
    );

Map<String, dynamic> _$GCRemoveUserArgsToJson(GCRemoveUserArgs instance) =>
    <String, dynamic>{
      'gc': instance.gc,
      'uid': instance.uid,
    };

AddressBookEntry _$AddressBookEntryFromJson(Map<String, dynamic> json) =>
    AddressBookEntry(
      json['id'] as String,
      json['nick'] as String,
      json['name'] as String,
      json['ignored'] as bool,
      DateTime.parse(json['first_created'] as String),
      DateTime.parse(json['last_handshake_attempt'] as String),
      base64ToUint8list(json['avatar'] as String?),
      DateTime.parse(json['last_completed_kx'] as String),
    );

Map<String, dynamic> _$AddressBookEntryToJson(AddressBookEntry instance) =>
    <String, dynamic>{
      'id': instance.id,
      'nick': instance.nick,
      'name': instance.name,
      'ignored': instance.ignored,
      'first_created': instance.firstCreated.toIso8601String(),
      'last_handshake_attempt': instance.lastHandshakeAttempt.toIso8601String(),
      'avatar': instance.avatar,
      'last_completed_kx': instance.lastCompletedKx.toIso8601String(),
    };

ShareFileArgs _$ShareFileArgsFromJson(Map<String, dynamic> json) =>
    ShareFileArgs(
      json['filename'] as String,
      json['uid'] as String,
      json['cost'] as int,
      json['description'] as String,
    );

Map<String, dynamic> _$ShareFileArgsToJson(ShareFileArgs instance) =>
    <String, dynamic>{
      'filename': instance.filename,
      'uid': instance.uid,
      'cost': instance.cost,
      'description': instance.description,
    };

UnshareFileArgs _$UnshareFileArgsFromJson(Map<String, dynamic> json) =>
    UnshareFileArgs(
      json['fid'] as String,
      json['uid'] as String?,
    );

Map<String, dynamic> _$UnshareFileArgsToJson(UnshareFileArgs instance) {
  final val = <String, dynamic>{
    'fid': instance.fid,
  };

  void writeNotNull(String key, dynamic value) {
    if (value != null) {
      val[key] = value;
    }
  }

  writeNotNull('uid', instance.uid);
  return val;
}

GetRemoteFileArgs _$GetRemoteFileArgsFromJson(Map<String, dynamic> json) =>
    GetRemoteFileArgs(
      json['uid'] as String,
      json['fid'] as String,
    );

Map<String, dynamic> _$GetRemoteFileArgsToJson(GetRemoteFileArgs instance) =>
    <String, dynamic>{
      'uid': instance.uid,
      'fid': instance.fid,
    };

FileManifest _$FileManifestFromJson(Map<String, dynamic> json) => FileManifest(
      json['index'] as int,
      json['size'] as int,
      json['hash'] as String,
    );

Map<String, dynamic> _$FileManifestToJson(FileManifest instance) =>
    <String, dynamic>{
      'index': instance.index,
      'size': instance.size,
      'hash': instance.hash,
    };

FileMetadata _$FileMetadataFromJson(Map<String, dynamic> json) => FileMetadata(
      json['version'] as int,
      json['cost'] as int,
      json['size'] as int,
      json['directory'] as String,
      json['filename'] as String,
      json['description'] as String,
      json['hash'] as String,
      (json['manifest'] as List<dynamic>)
          .map((e) => FileManifest.fromJson(e as Map<String, dynamic>))
          .toList(),
      json['signature'] as String,
      json['attributes'] as Map<String, dynamic>?,
    );

Map<String, dynamic> _$FileMetadataToJson(FileMetadata instance) =>
    <String, dynamic>{
      'version': instance.version,
      'cost': instance.cost,
      'size': instance.size,
      'directory': instance.directory,
      'filename': instance.filename,
      'description': instance.description,
      'hash': instance.hash,
      'manifest': instance.manifest,
      'signature': instance.signature,
      'attributes': instance.attributes,
    };

SharedFile _$SharedFileFromJson(Map<String, dynamic> json) => SharedFile(
      json['file_hash'] as String,
      json['fid'] as String,
      json['filename'] as String,
    );

Map<String, dynamic> _$SharedFileToJson(SharedFile instance) =>
    <String, dynamic>{
      'file_hash': instance.fileHash,
      'fid': instance.fid,
      'filename': instance.filename,
    };

SharedFileAndShares _$SharedFileAndSharesFromJson(Map<String, dynamic> json) =>
    SharedFileAndShares(
      SharedFile.fromJson(json['shared_file'] as Map<String, dynamic>),
      json['cost'] as int,
      json['size'] as int,
      json['global'] as bool,
      (json['shares'] as List<dynamic>).map((e) => e as String).toList(),
    );

Map<String, dynamic> _$SharedFileAndSharesToJson(
        SharedFileAndShares instance) =>
    <String, dynamic>{
      'shared_file': instance.sf,
      'cost': instance.cost,
      'size': instance.size,
      'global': instance.global,
      'shares': instance.shares,
    };

ReceivedFile _$ReceivedFileFromJson(Map<String, dynamic> json) => ReceivedFile(
      json['file_id'] as String,
      json['uid'] as String,
      json['disk_path'] as String,
      json['metadata'] == null
          ? null
          : FileMetadata.fromJson(json['metadata'] as Map<String, dynamic>),
    );

Map<String, dynamic> _$ReceivedFileToJson(ReceivedFile instance) =>
    <String, dynamic>{
      'file_id': instance.fid,
      'uid': instance.uid,
      'disk_path': instance.diskPath,
      'metadata': instance.metadata,
    };

UserContentList _$UserContentListFromJson(Map<String, dynamic> json) =>
    UserContentList(
      json['uid'] as String,
      (json['files'] as List<dynamic>)
          .map((e) => ReceivedFile.fromJson(e as Map<String, dynamic>))
          .toList(),
    );

Map<String, dynamic> _$UserContentListToJson(UserContentList instance) =>
    <String, dynamic>{
      'uid': instance.uid,
      'files': instance.files,
    };

PayTipArgs _$PayTipArgsFromJson(Map<String, dynamic> json) => PayTipArgs(
      json['uid'] as String,
      (json['amount'] as num).toDouble(),
    );

Map<String, dynamic> _$PayTipArgsToJson(PayTipArgs instance) =>
    <String, dynamic>{
      'uid': instance.uid,
      'amount': instance.amount,
    };

PostMetadata _$PostMetadataFromJson(Map<String, dynamic> json) => PostMetadata(
      json['version'] as int,
      Map<String, String>.from(json['attributes'] as Map),
    );

Map<String, dynamic> _$PostMetadataToJson(PostMetadata instance) =>
    <String, dynamic>{
      'version': instance.version,
      'attributes': instance.attributes,
    };

PostMetadataStatus _$PostMetadataStatusFromJson(Map<String, dynamic> json) =>
    PostMetadataStatus(
      json['version'] as int,
      json['from'] as String,
      json['link'] as String,
      Map<String, String>.from(json['attributes'] as Map),
    );

Map<String, dynamic> _$PostMetadataStatusToJson(PostMetadataStatus instance) =>
    <String, dynamic>{
      'version': instance.version,
      'from': instance.from,
      'link': instance.link,
      'attributes': instance.attributes,
    };

PostReceived _$PostReceivedFromJson(Map<String, dynamic> json) => PostReceived(
      json['uid'] as String,
      PostMetadata.fromJson(json['post_meta'] as Map<String, dynamic>),
    );

Map<String, dynamic> _$PostReceivedToJson(PostReceived instance) =>
    <String, dynamic>{
      'uid': instance.uid,
      'post_meta': instance.postMeta,
    };

PostSummary _$PostSummaryFromJson(Map<String, dynamic> json) => PostSummary(
      json['id'] as String,
      json['from'] as String,
      json['author_id'] as String,
      json['author_nick'] as String,
      DateTime.parse(json['date'] as String),
      DateTime.parse(json['last_status_ts'] as String),
      json['title'] as String,
    );

Map<String, dynamic> _$PostSummaryToJson(PostSummary instance) =>
    <String, dynamic>{
      'id': instance.id,
      'from': instance.from,
      'author_id': instance.authorID,
      'author_nick': instance.authorNick,
      'date': instance.date.toIso8601String(),
      'last_status_ts': instance.lastStatusTS.toIso8601String(),
      'title': instance.title,
    };

ReadPostArgs _$ReadPostArgsFromJson(Map<String, dynamic> json) => ReadPostArgs(
      json['from'] as String,
      json['pid'] as String,
    );

Map<String, dynamic> _$ReadPostArgsToJson(ReadPostArgs instance) =>
    <String, dynamic>{
      'from': instance.from,
      'pid': instance.pid,
    };

CommentPostArgs _$CommentPostArgsFromJson(Map<String, dynamic> json) =>
    CommentPostArgs(
      json['from'] as String,
      json['pid'] as String,
      json['comment'] as String,
      json['parent'] as String?,
    );

Map<String, dynamic> _$CommentPostArgsToJson(CommentPostArgs instance) =>
    <String, dynamic>{
      'from': instance.from,
      'pid': instance.pid,
      'comment': instance.comment,
      'parent': instance.parent,
    };

PostStatusReceived _$PostStatusReceivedFromJson(Map<String, dynamic> json) =>
    PostStatusReceived(
      json['post_from'] as String,
      json['pid'] as String,
      json['status_from'] as String,
      PostMetadataStatus.fromJson(json['status'] as Map<String, dynamic>),
      json['mine'] as bool,
    );

Map<String, dynamic> _$PostStatusReceivedToJson(PostStatusReceived instance) =>
    <String, dynamic>{
      'post_from': instance.postFrom,
      'pid': instance.pid,
      'status_from': instance.statusFrom,
      'status': instance.status,
      'mine': instance.mine,
    };

MediateIDArgs _$MediateIDArgsFromJson(Map<String, dynamic> json) =>
    MediateIDArgs(
      json['mediator'] as String,
      json['target'] as String,
    );

Map<String, dynamic> _$MediateIDArgsToJson(MediateIDArgs instance) =>
    <String, dynamic>{
      'mediator': instance.mediator,
      'target': instance.target,
    };

PostActionArgs _$PostActionArgsFromJson(Map<String, dynamic> json) =>
    PostActionArgs(
      json['from'] as String,
      json['pid'] as String,
    );

Map<String, dynamic> _$PostActionArgsToJson(PostActionArgs instance) =>
    <String, dynamic>{
      'from': instance.from,
      'pid': instance.pid,
    };

FileDownload _$FileDownloadFromJson(Map<String, dynamic> json) => FileDownload(
      json['uid'] as String,
      json['fid'] as String,
      json['completed_name'] as String,
      json['metadata'] == null
          ? null
          : FileMetadata.fromJson(json['metadata'] as Map<String, dynamic>),
      (json['invoices'] as Map<String, dynamic>?)?.map(
            (k, e) => MapEntry(int.parse(k), e as String),
          ) ??
          {},
      (json['chunkstates'] as Map<String, dynamic>?)?.map(
            (k, e) => MapEntry(int.parse(k), e as String),
          ) ??
          {},
      json['disk_path'] as String,
    );

Map<String, dynamic> _$FileDownloadToJson(FileDownload instance) =>
    <String, dynamic>{
      'uid': instance.uid,
      'fid': instance.fid,
      'completed_name': instance.completedName,
      'metadata': instance.metadata,
      'invoices': instance.invoices.map((k, e) => MapEntry(k.toString(), e)),
      'chunkstates':
          instance.chunkStates.map((k, e) => MapEntry(k.toString(), e)),
      'disk_path': instance.diskPath,
    };

FileDownloadProgress _$FileDownloadProgressFromJson(
        Map<String, dynamic> json) =>
    FileDownloadProgress(
      json['uid'] as String,
      json['fid'] as String,
      FileMetadata.fromJson(json['metadata'] as Map<String, dynamic>),
      json['nb_missing_chunks'] as int,
    );

Map<String, dynamic> _$FileDownloadProgressToJson(
        FileDownloadProgress instance) =>
    <String, dynamic>{
      'uid': instance.uid,
      'fid': instance.fid,
      'metadata': instance.metadata,
      'nb_missing_chunks': instance.nbMissingChunks,
    };

LNChain _$LNChainFromJson(Map<String, dynamic> json) => LNChain(
      json['chain'] as String,
      json['network'] as String,
    );

Map<String, dynamic> _$LNChainToJson(LNChain instance) => <String, dynamic>{
      'chain': instance.chain,
      'network': instance.network,
    };

LNInfo _$LNInfoFromJson(Map<String, dynamic> json) => LNInfo(
      json['identity_pubkey'] as String,
      json['version'] as String,
      json['num_active_channels'] as int? ?? 0,
      json['num_inactive_channels'] as int? ?? 0,
      json['num_pending_channels'] as int? ?? 0,
      json['synced_to_chain'] as bool? ?? false,
      json['synced_to_graph'] as bool? ?? false,
      json['block_height'] as int,
      json['block_hash'] as String,
      (json['chains'] as List<dynamic>)
          .map((e) => LNChain.fromJson(e as Map<String, dynamic>))
          .toList(),
    );

Map<String, dynamic> _$LNInfoToJson(LNInfo instance) => <String, dynamic>{
      'identity_pubkey': instance.identityPubkey,
      'version': instance.version,
      'num_active_channels': instance.numActiveChannels,
      'num_inactive_channels': instance.numInactiveChannels,
      'num_pending_channels': instance.numPendingChannels,
      'synced_to_chain': instance.syncedToChain,
      'synced_to_graph': instance.syncedToGraph,
      'block_height': instance.blockHeight,
      'block_hash': instance.blockHash,
      'chains': instance.chains,
    };

LNChannel _$LNChannelFromJson(Map<String, dynamic> json) => LNChannel(
      json['active'] as bool? ?? false,
      json['remote_pubkey'] as String,
      json['channel_point'] as String,
      json['chan_id'] as int,
      json['capacity'] as int,
      json['local_balance'] as int? ?? 0,
      json['remote_balance'] as int? ?? 0,
      json['short_chan_id'] as String,
    );

Map<String, dynamic> _$LNChannelToJson(LNChannel instance) => <String, dynamic>{
      'active': instance.active,
      'remote_pubkey': instance.remotePubkey,
      'channel_point': instance.channelPoint,
      'chan_id': instance.chanID,
      'capacity': instance.capacity,
      'local_balance': instance.localBalance,
      'remote_balance': instance.remoteBalance,
      'short_chan_id': instance.shortChanID,
    };

LNPendingChannel _$LNPendingChannelFromJson(Map<String, dynamic> json) =>
    LNPendingChannel(
      json['remote_node_pub'] as String,
      json['channel_point'] as String,
      json['capacity'] as int,
      json['local_balance'] as int? ?? 0,
      json['remote_balance'] as int? ?? 0,
      json['initiator'] as int? ?? 0,
      json['short_chan_id'] as String,
    );

Map<String, dynamic> _$LNPendingChannelToJson(LNPendingChannel instance) =>
    <String, dynamic>{
      'remote_node_pub': instance.remoteNodePub,
      'channel_point': instance.channelPoint,
      'capacity': instance.capacity,
      'local_balance': instance.localBalance,
      'remote_balance': instance.remoteBalance,
      'initiator': instance.initiator,
      'short_chan_id': instance.shortChanID,
    };

LNPendingOpenChannel _$LNPendingOpenChannelFromJson(
        Map<String, dynamic> json) =>
    LNPendingOpenChannel(
      json['confirmation_height'] as int? ?? 0,
      json['commit_fee'] as int? ?? 0,
      json['confirmation_size'] as int? ?? 0,
      json['fee_per_kb'] as int? ?? 0,
      LNPendingChannel.fromJson(json['channel'] as Map<String, dynamic>),
    );

Map<String, dynamic> _$LNPendingOpenChannelToJson(
        LNPendingOpenChannel instance) =>
    <String, dynamic>{
      'channel': instance.channel,
      'confirmation_height': instance.confirmationHeight,
      'commit_fee': instance.commitFee,
      'confirmation_size': instance.commitSize,
      'fee_per_kb': instance.feePerKb,
    };

LNWaitingCloseChannel _$LNWaitingCloseChannelFromJson(
        Map<String, dynamic> json) =>
    LNWaitingCloseChannel(
      LNPendingChannel.fromJson(json['channel'] as Map<String, dynamic>),
    );

Map<String, dynamic> _$LNWaitingCloseChannelToJson(
        LNWaitingCloseChannel instance) =>
    <String, dynamic>{
      'channel': instance.channel,
    };

LNPendingForceClosingChannel _$LNPendingForceClosingChannelFromJson(
        Map<String, dynamic> json) =>
    LNPendingForceClosingChannel(
      LNPendingChannel.fromJson(json['channel'] as Map<String, dynamic>),
      json['closing_txid'] as String? ?? '',
      json['maturityHeight'] as int? ?? 0,
      json['blocksTilMaturity'] as int? ?? 0,
      json['recoveredBalance'] as int? ?? 0,
    );

Map<String, dynamic> _$LNPendingForceClosingChannelToJson(
        LNPendingForceClosingChannel instance) =>
    <String, dynamic>{
      'channel': instance.channel,
      'closing_txid': instance.closingTxid,
      'maturityHeight': instance.maturityHeight,
      'blocksTilMaturity': instance.blocksTilMaturity,
      'recoveredBalance': instance.recoveredBalance,
    };

LNPendingChannelsList _$LNPendingChannelsListFromJson(
        Map<String, dynamic> json) =>
    LNPendingChannelsList(
      (json['pending_open_channels'] as List<dynamic>?)
              ?.map((e) =>
                  LNPendingOpenChannel.fromJson(e as Map<String, dynamic>))
              .toList() ??
          [],
      (json['pending_force_closing_channels'] as List<dynamic>?)
              ?.map((e) => LNPendingForceClosingChannel.fromJson(
                  e as Map<String, dynamic>))
              .toList() ??
          [],
      (json['waiting_close_channels'] as List<dynamic>?)
              ?.map((e) =>
                  LNWaitingCloseChannel.fromJson(e as Map<String, dynamic>))
              .toList() ??
          [],
    );

Map<String, dynamic> _$LNPendingChannelsListToJson(
        LNPendingChannelsList instance) =>
    <String, dynamic>{
      'pending_open_channels': instance.pendingOpen,
      'pending_force_closing_channels': instance.pendingForceClose,
      'waiting_close_channels': instance.waitingClose,
    };

LNGenInvoiceRequest _$LNGenInvoiceRequestFromJson(Map<String, dynamic> json) =>
    LNGenInvoiceRequest(
      json['memo'] as String,
      json['value'] as int,
    );

Map<String, dynamic> _$LNGenInvoiceRequestToJson(
        LNGenInvoiceRequest instance) =>
    <String, dynamic>{
      'memo': instance.memo,
      'value': instance.value,
    };

LNGenInvoiceResponse _$LNGenInvoiceResponseFromJson(
        Map<String, dynamic> json) =>
    LNGenInvoiceResponse(
      json['r_hash'] as String,
      json['payment_request'] as String,
    );

Map<String, dynamic> _$LNGenInvoiceResponseToJson(
        LNGenInvoiceResponse instance) =>
    <String, dynamic>{
      'r_hash': instance.rhash,
      'payment_request': instance.paymentRequest,
    };

LNPayInvoiceResponse _$LNPayInvoiceResponseFromJson(
        Map<String, dynamic> json) =>
    LNPayInvoiceResponse(
      json['payment_error'] as String? ?? '',
      base64ToHex(json['payment_preimage'] as String),
      base64ToHex(json['payment_hash'] as String),
    );

Map<String, dynamic> _$LNPayInvoiceResponseToJson(
        LNPayInvoiceResponse instance) =>
    <String, dynamic>{
      'payment_error': instance.paymentError,
      'payment_preimage': instance.preimage,
      'payment_hash': instance.paymentHash,
    };

LNQueryRouteRequest _$LNQueryRouteRequestFromJson(Map<String, dynamic> json) =>
    LNQueryRouteRequest(
      json['pub_key'] as String,
      json['amt'] as int,
    );

Map<String, dynamic> _$LNQueryRouteRequestToJson(
        LNQueryRouteRequest instance) =>
    <String, dynamic>{
      'pub_key': instance.pubkey,
      'amt': instance.amount,
    };

LNHop _$LNHopFromJson(Map<String, dynamic> json) => LNHop(
      json['chan_id'] as int? ?? 0,
      json['chan_capacity'] as int? ?? 0,
      json['pub_key'] as String,
    );

Map<String, dynamic> _$LNHopToJson(LNHop instance) => <String, dynamic>{
      'chan_id': instance.chanId,
      'chan_capacity': instance.chanCapacity,
      'pub_key': instance.pubkey,
    };

LNRoute _$LNRouteFromJson(Map<String, dynamic> json) => LNRoute(
      json['total_time_lock'] as int,
      json['total_fees'] as int? ?? 0,
      (json['hops'] as List<dynamic>?)
              ?.map((e) => LNHop.fromJson(e as Map<String, dynamic>))
              .toList() ??
          [],
    );

Map<String, dynamic> _$LNRouteToJson(LNRoute instance) => <String, dynamic>{
      'total_time_lock': instance.totalTimeLock,
      'total_fees': instance.totalFees,
      'hops': instance.hops,
    };

LNQueryRouteResponse _$LNQueryRouteResponseFromJson(
        Map<String, dynamic> json) =>
    LNQueryRouteResponse(
      (json['routes'] as List<dynamic>?)
              ?.map((e) => LNRoute.fromJson(e as Map<String, dynamic>))
              .toList() ??
          [],
      (json['success_prob'] as num?)?.toDouble() ?? 0,
    );

Map<String, dynamic> _$LNQueryRouteResponseToJson(
        LNQueryRouteResponse instance) =>
    <String, dynamic>{
      'routes': instance.routes,
      'success_prob': instance.successProb,
    };

LNGetNodeInfoRequest _$LNGetNodeInfoRequestFromJson(
        Map<String, dynamic> json) =>
    LNGetNodeInfoRequest(
      json['pub_key'] as String,
      json['include_channels'] as bool? ?? false,
    );

Map<String, dynamic> _$LNGetNodeInfoRequestToJson(
        LNGetNodeInfoRequest instance) =>
    <String, dynamic>{
      'pub_key': instance.pubkey,
      'include_channels': instance.includeChannels,
    };

LNNode _$LNNodeFromJson(Map<String, dynamic> json) => LNNode(
      json['pub_key'] as String,
      json['alias'] as String? ?? '',
      json['last_update'] as int? ?? 0,
    );

Map<String, dynamic> _$LNNodeToJson(LNNode instance) => <String, dynamic>{
      'pub_key': instance.pubkey,
      'alias': instance.alias,
      'last_update': instance.lastUpdate,
    };

LNRoutingPolicy _$LNRoutingPolicyFromJson(Map<String, dynamic> json) =>
    LNRoutingPolicy(
      json['disabled'] as bool? ?? false,
      json['last_update'] as int? ?? 0,
    );

Map<String, dynamic> _$LNRoutingPolicyToJson(LNRoutingPolicy instance) =>
    <String, dynamic>{
      'disabled': instance.disabled,
      'last_update': instance.lastUpdate,
    };

LNChannelEdge _$LNChannelEdgeFromJson(Map<String, dynamic> json) =>
    LNChannelEdge(
      json['channel_id'] as int? ?? 0,
      json['chan_point'] as String,
      json['last_update'] as int? ?? 0,
      json['node1_pub'] as String,
      json['node2_pub'] as String,
      json['capacity'] as int? ?? 0,
      LNRoutingPolicy.fromJson(json['node1_policy'] as Map<String, dynamic>),
      LNRoutingPolicy.fromJson(json['node2_policy'] as Map<String, dynamic>),
    );

Map<String, dynamic> _$LNChannelEdgeToJson(LNChannelEdge instance) =>
    <String, dynamic>{
      'channel_id': instance.channelID,
      'chan_point': instance.channelPoint,
      'last_update': instance.lastUpdate,
      'node1_pub': instance.node1Pub,
      'node2_pub': instance.node2Pub,
      'capacity': instance.capacity,
      'node1_policy': instance.node1Policy,
      'node2_policy': instance.node2Policy,
    };

LNGetNodeInfoResponse _$LNGetNodeInfoResponseFromJson(
        Map<String, dynamic> json) =>
    LNGetNodeInfoResponse(
      LNNode.fromJson(json['node'] as Map<String, dynamic>),
      json['num_channels'] as int,
      json['total_capacity'] as int? ?? 0,
      (json['channels'] as List<dynamic>?)
              ?.map((e) => LNChannelEdge.fromJson(e as Map<String, dynamic>))
              .toList() ??
          [],
    );

Map<String, dynamic> _$LNGetNodeInfoResponseToJson(
        LNGetNodeInfoResponse instance) =>
    <String, dynamic>{
      'node': instance.node,
      'num_channels': instance.numChannels,
      'total_capacity': instance.totalCapacity,
      'channels': instance.channels,
    };

LNChannelBalance _$LNChannelBalanceFromJson(Map<String, dynamic> json) =>
    LNChannelBalance(
      json['balance'] as int? ?? 0,
      json['pending_open_balance'] as int? ?? 0,
      json['max_inbound_amount'] as int? ?? 0,
      json['max_outbound_amount'] as int? ?? 0,
    );

Map<String, dynamic> _$LNChannelBalanceToJson(LNChannelBalance instance) =>
    <String, dynamic>{
      'balance': instance.balance,
      'pending_open_balance': instance.pendingOpenBalance,
      'max_inbound_amount': instance.maxInboundAmount,
      'max_outbound_amount': instance.maxOutboundAmount,
    };

LNWalletBalance _$LNWalletBalanceFromJson(Map<String, dynamic> json) =>
    LNWalletBalance(
      json['total_balance'] as int? ?? 0,
      json['confirmed_balance'] as int? ?? 0,
      json['unconfirmed_balance'] as int? ?? 0,
    );

Map<String, dynamic> _$LNWalletBalanceToJson(LNWalletBalance instance) =>
    <String, dynamic>{
      'total_balance': instance.totalBalance,
      'confirmed_balance': instance.confirmedBalance,
      'unconfirmed_balance': instance.unconfirmedBalance,
    };

LNBalances _$LNBalancesFromJson(Map<String, dynamic> json) => LNBalances(
      LNChannelBalance.fromJson(json['channel'] as Map<String, dynamic>),
      LNWalletBalance.fromJson(json['wallet'] as Map<String, dynamic>),
    );

Map<String, dynamic> _$LNBalancesToJson(LNBalances instance) =>
    <String, dynamic>{
      'channel': instance.channel,
      'wallet': instance.wallet,
    };

LNDecodedInvoice _$LNDecodedInvoiceFromJson(Map<String, dynamic> json) =>
    LNDecodedInvoice(
      json['destination'] as String,
      json['payment_hash'] as String,
      json['numAtoms'] as int? ?? 0,
      json['expiry'] as int? ?? 3600,
      json['description'] as String? ?? '',
      json['timestamp'] as int? ?? 0,
      json['num_m_atoms'] as int? ?? 0,
    );

Map<String, dynamic> _$LNDecodedInvoiceToJson(LNDecodedInvoice instance) =>
    <String, dynamic>{
      'destination': instance.destination,
      'payment_hash': instance.paymentHash,
      'numAtoms': instance.numAtoms,
      'num_m_atoms': instance.numMAtoms,
      'expiry': instance.expiry,
      'description': instance.description,
      'timestamp': instance.timestamp,
    };

LNPayInvoiceRequest _$LNPayInvoiceRequestFromJson(Map<String, dynamic> json) =>
    LNPayInvoiceRequest(
      json['payment_request'] as String,
      json['amount'] as int,
    );

Map<String, dynamic> _$LNPayInvoiceRequestToJson(
        LNPayInvoiceRequest instance) =>
    <String, dynamic>{
      'payment_request': instance.paymentRequest,
      'amount': instance.amount,
    };

LNPeer _$LNPeerFromJson(Map<String, dynamic> json) => LNPeer(
      json['pub_key'] as String,
      json['address'] as String,
      json['inbound'] as bool? ?? false,
    );

Map<String, dynamic> _$LNPeerToJson(LNPeer instance) => <String, dynamic>{
      'pub_key': instance.pubkey,
      'address': instance.address,
      'inbound': instance.inbound,
    };

LNOpenChannelRequest _$LNOpenChannelRequestFromJson(
        Map<String, dynamic> json) =>
    LNOpenChannelRequest(
      json['node_pubkey'] as String,
      json['local_funding_amount'] as int,
      json['push_atoms'] as int,
    );

Map<String, dynamic> _$LNOpenChannelRequestToJson(
        LNOpenChannelRequest instance) =>
    <String, dynamic>{
      'node_pubkey': hexToBase64(instance.nodePubkey),
      'local_funding_amount': instance.localFundingAmount,
      'push_atoms': instance.pushAtoms,
    };

LNChannelPoint_FundingTxidStr _$LNChannelPoint_FundingTxidStrFromJson(
        Map<String, dynamic> json) =>
    LNChannelPoint_FundingTxidStr(
      json['fundingTxidStr'] as String? ?? '',
    );

Map<String, dynamic> _$LNChannelPoint_FundingTxidStrToJson(
        LNChannelPoint_FundingTxidStr instance) =>
    <String, dynamic>{
      'fundingTxidStr': instance.fundingTxidStr,
    };

LNChannelPoint_FundingTxidBytes _$LNChannelPoint_FundingTxidBytesFromJson(
        Map<String, dynamic> json) =>
    LNChannelPoint_FundingTxidBytes(
      json['fundingTxidBytes'] as String? ?? '',
    );

Map<String, dynamic> _$LNChannelPoint_FundingTxidBytesToJson(
        LNChannelPoint_FundingTxidBytes instance) =>
    <String, dynamic>{
      'fundingTxidBytes': instance.fundingTxidBytes,
    };

LNChannelPoint _$LNChannelPointFromJson(Map<String, dynamic> json) =>
    LNChannelPoint(
      json['txid'],
      json['output_index'] as int? ?? 0,
    );

Map<String, dynamic> _$LNChannelPointToJson(LNChannelPoint instance) =>
    <String, dynamic>{
      'txid': instance.txid,
      'output_index': instance.outputIndex,
    };

LNCloseChannelRequest _$LNCloseChannelRequestFromJson(
        Map<String, dynamic> json) =>
    LNCloseChannelRequest(
      LNChannelPoint.fromJson(json['channel_point'] as Map<String, dynamic>),
      json['force'] as bool,
    );

Map<String, dynamic> _$LNCloseChannelRequestToJson(
        LNCloseChannelRequest instance) =>
    <String, dynamic>{
      'channel_point': instance.channelPoint,
      'force': instance.force,
    };

LNTryExternalDcrlnd _$LNTryExternalDcrlndFromJson(Map<String, dynamic> json) =>
    LNTryExternalDcrlnd(
      json['rpc_host'] as String,
      json['tls_cert_path'] as String,
      json['macaroon_path'] as String,
    );

Map<String, dynamic> _$LNTryExternalDcrlndToJson(
        LNTryExternalDcrlnd instance) =>
    <String, dynamic>{
      'rpc_host': instance.rpcHost,
      'tls_cert_path': instance.tlsCertPath,
      'macaroon_path': instance.macaroonPath,
    };

LNInitDcrlnd _$LNInitDcrlndFromJson(Map<String, dynamic> json) => LNInitDcrlnd(
      json['root_dir'] as String,
      json['network'] as String,
      json['password'] as String,
      (json['existingSeed'] as List<dynamic>).map((e) => e as String).toList(),
      base64ToUint8list(json['multiChanBackup'] as String?),
      json['proxyaddr'] as String,
      json['torisolation'] as bool,
      json['proxy_username'] as String,
      json['proxy_password'] as String,
      json['circuit_limit'] as int,
      json['sync_free_list'] as bool,
      json['autocompact'] as bool,
      json['autocompact_min_age'] as int,
      json['debug_level'] as String,
    );

Map<String, dynamic> _$LNInitDcrlndToJson(LNInitDcrlnd instance) =>
    <String, dynamic>{
      'root_dir': instance.rootDir,
      'network': instance.network,
      'password': instance.password,
      'existingSeed': instance.existingSeed,
      'multiChanBackup': uint8listToBase64(instance.multiChanBackup),
      'proxyaddr': instance.proxyaddr,
      'torisolation': instance.torIsolation,
      'proxy_username': instance.proxyUsername,
      'proxy_password': instance.proxyPassword,
      'circuit_limit': instance.circuitLimit,
      'sync_free_list': instance.syncFreeList,
      'autocompact': instance.autoCompact,
      'autocompact_min_age': instance.autoCompactMinAge,
      'debug_level': instance.debugLevel,
    };

LNNewWalletSeed _$LNNewWalletSeedFromJson(Map<String, dynamic> json) =>
    LNNewWalletSeed(
      json['seed'] as String,
      json['rpc_host'] as String,
    );

Map<String, dynamic> _$LNNewWalletSeedToJson(LNNewWalletSeed instance) =>
    <String, dynamic>{
      'seed': instance.seed,
      'rpc_host': instance.rpcHost,
    };

LNInitialChainSyncUpdate _$LNInitialChainSyncUpdateFromJson(
        Map<String, dynamic> json) =>
    LNInitialChainSyncUpdate(
      json['block_height'] as int? ?? 0,
      base64ToHexReversed(json['block_hash'] as String),
      json['block_timestamp'] as int? ?? 0,
      json['synced'] as bool? ?? false,
    );

Map<String, dynamic> _$LNInitialChainSyncUpdateToJson(
        LNInitialChainSyncUpdate instance) =>
    <String, dynamic>{
      'block_height': instance.blockHeight,
      'block_hash': instance.blockHash,
      'block_timestamp': instance.blockTimestamp,
      'synced': instance.synced,
    };

LNReqChannelArgs _$LNReqChannelArgsFromJson(Map<String, dynamic> json) =>
    LNReqChannelArgs(
      json['server'] as String,
      json['key'] as String,
      json['chan_size'] as int,
      json['certificates'] as String,
    );

Map<String, dynamic> _$LNReqChannelArgsToJson(LNReqChannelArgs instance) =>
    <String, dynamic>{
      'server': instance.server,
      'key': instance.key,
      'chan_size': instance.chanSize,
      'certificates': instance.certificates,
    };

LNLPPolicyResponse _$LNLPPolicyResponseFromJson(Map<String, dynamic> json) =>
    LNLPPolicyResponse(
      json['node'] as String,
      (json['addresses'] as List<dynamic>).map((e) => e as String).toList(),
      json['min_chan_size'] as int,
      json['max_chan_size'] as int,
      json['max_nb_channels'] as int,
      json['min_chan_lifetime'] as int,
      (json['chan_invoice_fee_rate'] as num).toDouble(),
    );

Map<String, dynamic> _$LNLPPolicyResponseToJson(LNLPPolicyResponse instance) =>
    <String, dynamic>{
      'node': instance.node,
      'addresses': instance.addresses,
      'min_chan_size': instance.minChanSize,
      'max_chan_size': instance.maxChanSize,
      'max_nb_channels': instance.maxNbChannels,
      'min_chan_lifetime': instance.minChanLifetime,
      'chan_invoice_fee_rate': instance.chanInvoiceFeeRate,
    };

LNReqChannelEstValue _$LNReqChannelEstValueFromJson(
        Map<String, dynamic> json) =>
    LNReqChannelEstValue(
      json['amount'] as int? ?? 0,
      LNLPPolicyResponse.fromJson(
          json['server_policy'] as Map<String, dynamic>),
      LNReqChannelArgs.fromJson(json['request'] as Map<String, dynamic>),
    );

Map<String, dynamic> _$LNReqChannelEstValueToJson(
        LNReqChannelEstValue instance) =>
    <String, dynamic>{
      'amount': instance.amount,
      'server_policy': instance.serverPolicy,
      'request': instance.request,
    };

ConfirmFileDownload _$ConfirmFileDownloadFromJson(Map<String, dynamic> json) =>
    ConfirmFileDownload(
      json['uid'] as String,
      json['fid'] as String,
      FileMetadata.fromJson(json['metadata'] as Map<String, dynamic>),
    );

Map<String, dynamic> _$ConfirmFileDownloadToJson(
        ConfirmFileDownload instance) =>
    <String, dynamic>{
      'uid': instance.uid,
      'fid': instance.fid,
      'metadata': instance.metadata,
    };

ConfirmFileDownloadReply _$ConfirmFileDownloadReplyFromJson(
        Map<String, dynamic> json) =>
    ConfirmFileDownloadReply(
      json['fid'] as String,
      json['reply'] as bool,
    );

Map<String, dynamic> _$ConfirmFileDownloadReplyToJson(
        ConfirmFileDownloadReply instance) =>
    <String, dynamic>{
      'fid': instance.fid,
      'reply': instance.reply,
    };

SendFileArgs _$SendFileArgsFromJson(Map<String, dynamic> json) => SendFileArgs(
      json['uid'] as String,
      json['filepath'] as String,
    );

Map<String, dynamic> _$SendFileArgsToJson(SendFileArgs instance) =>
    <String, dynamic>{
      'uid': instance.uid,
      'filepath': instance.filepath,
    };

UserPayStats _$UserPayStatsFromJson(Map<String, dynamic> json) => UserPayStats(
      json['total_sent'] as int,
      json['total_received'] as int,
    );

Map<String, dynamic> _$UserPayStatsToJson(UserPayStats instance) =>
    <String, dynamic>{
      'total_sent': instance.totalSent,
      'total_received': instance.totalReceived,
    };

PayStatsSummary _$PayStatsSummaryFromJson(Map<String, dynamic> json) =>
    PayStatsSummary(
      json['prefix'] as String,
      json['total'] as int,
    );

Map<String, dynamic> _$PayStatsSummaryToJson(PayStatsSummary instance) =>
    <String, dynamic>{
      'prefix': instance.prefix,
      'total': instance.total,
    };

PostListItem _$PostListItemFromJson(Map<String, dynamic> json) => PostListItem(
      json['id'] as String,
      json['title'] as String,
      json['timestamp'] as int? ?? 0,
    );

Map<String, dynamic> _$PostListItemToJson(PostListItem instance) =>
    <String, dynamic>{
      'id': instance.id,
      'title': instance.title,
      'timestamp': instance.timestamp,
    };

UserPostList _$UserPostListFromJson(Map<String, dynamic> json) => UserPostList(
      json['uid'] as String,
      (json['posts'] as List<dynamic>)
          .map((e) => PostListItem.fromJson(e as Map<String, dynamic>))
          .toList(),
    );

Map<String, dynamic> _$UserPostListToJson(UserPostList instance) =>
    <String, dynamic>{
      'uid': instance.uid,
      'posts': instance.posts,
    };

LocalRenameArgs _$LocalRenameArgsFromJson(Map<String, dynamic> json) =>
    LocalRenameArgs(
      json['id'] as String,
      json['new_name'] as String,
      json['is_gc'] as bool? ?? false,
    );

Map<String, dynamic> _$LocalRenameArgsToJson(LocalRenameArgs instance) =>
    <String, dynamic>{
      'id': instance.id,
      'new_name': instance.newName,
      'is_gc': instance.isGC,
    };

PostSubscriptionResult _$PostSubscriptionResultFromJson(
        Map<String, dynamic> json) =>
    PostSubscriptionResult(
      json['id'] as String,
      json['was_sub_request'] as bool,
      json['error'] as String,
    );

Map<String, dynamic> _$PostSubscriptionResultToJson(
        PostSubscriptionResult instance) =>
    <String, dynamic>{
      'id': instance.id,
      'was_sub_request': instance.wasSubRequest,
      'error': instance.error,
    };

PostSubscriberUpdated _$PostSubscriberUpdatedFromJson(
        Map<String, dynamic> json) =>
    PostSubscriberUpdated(
      json['id'] as String,
      json['nick'] as String,
      json['subscribed'] as bool,
    );

Map<String, dynamic> _$PostSubscriberUpdatedToJson(
        PostSubscriberUpdated instance) =>
    <String, dynamic>{
      'id': instance.id,
      'nick': instance.nick,
      'subscribed': instance.subscribed,
    };

LastUserReceivedTime _$LastUserReceivedTimeFromJson(
        Map<String, dynamic> json) =>
    LastUserReceivedTime(
      json['uid'] as String,
      json['last_decrypted'] as int,
    );

Map<String, dynamic> _$LastUserReceivedTimeToJson(
        LastUserReceivedTime instance) =>
    <String, dynamic>{
      'uid': instance.uid,
      'last_decrypted': instance.lastDecrypted,
    };

RatchetDebugInfo _$RatchetDebugInfoFromJson(Map<String, dynamic> json) =>
    RatchetDebugInfo(
      json['send_rv'] as String,
      json['send_rv_plain'] as String,
      json['recv_rv'] as String,
      json['recv_rv_plain'] as String,
      json['drain_rv'] as String,
      json['drain_rv_plain'] as String,
      json['my_reset_rv'] as String,
      json['their_reset_rv'] as String,
      json['nb_saved_keys'] as int,
      json['will_ratchet'] as bool,
      DateTime.parse(json['last_enc_time'] as String),
      DateTime.parse(json['last_dec_time'] as String),
    );

Map<String, dynamic> _$RatchetDebugInfoToJson(RatchetDebugInfo instance) =>
    <String, dynamic>{
      'send_rv': instance.sendRV,
      'send_rv_plain': instance.sendRVPlain,
      'recv_rv': instance.recvRV,
      'recv_rv_plain': instance.recvRVPlain,
      'drain_rv': instance.drainRV,
      'drain_rv_plain': instance.drainRVPlain,
      'my_reset_rv': instance.myResetRV,
      'their_reset_rv': instance.theirResetRV,
      'nb_saved_keys': instance.nbSavedKeys,
      'will_ratchet': instance.willRatchet,
      'last_enc_time': instance.lastEncTime.toIso8601String(),
      'last_dec_time': instance.lastDecTime.toIso8601String(),
    };

InvoiceGenFailed _$InvoiceGenFailedFromJson(Map<String, dynamic> json) =>
    InvoiceGenFailed(
      json['uid'] as String,
      json['nick'] as String,
      (json['dcr_amount'] as num?)?.toDouble() ?? 0,
      json['err'] as String,
    );

Map<String, dynamic> _$InvoiceGenFailedToJson(InvoiceGenFailed instance) =>
    <String, dynamic>{
      'uid': instance.uid,
      'nick': instance.nick,
      'dcr_amount': instance.dcrAmount,
      'err': instance.err,
    };

GCVersionWarn _$GCVersionWarnFromJson(Map<String, dynamic> json) =>
    GCVersionWarn(
      json['id'] as String,
      json['alias'] as String? ?? '',
      json['version'] as int,
      json['min_version'] as int,
      json['max_version'] as int,
    );

Map<String, dynamic> _$GCVersionWarnToJson(GCVersionWarn instance) =>
    <String, dynamic>{
      'id': instance.id,
      'alias': instance.alias,
      'version': instance.version,
      'min_version': instance.minVersion,
      'max_version': instance.maxVersion,
    };

GCAddedMembers _$GCAddedMembersFromJson(Map<String, dynamic> json) =>
    GCAddedMembers(
      json['id'] as String,
      (json['uids'] as List<dynamic>).map((e) => e as String).toList(),
    );

Map<String, dynamic> _$GCAddedMembersToJson(GCAddedMembers instance) =>
    <String, dynamic>{
      'id': instance.id,
      'uids': instance.uids,
    };

GCUpgradedVersion _$GCUpgradedVersionFromJson(Map<String, dynamic> json) =>
    GCUpgradedVersion(
      json['id'] as String,
      json['old_version'] as int? ?? 0,
      json['new_version'] as int? ?? 0,
    );

Map<String, dynamic> _$GCUpgradedVersionToJson(GCUpgradedVersion instance) =>
    <String, dynamic>{
      'id': instance.id,
      'old_version': instance.oldVersion,
      'new_version': instance.newVersion,
    };

GCMemberParted _$GCMemberPartedFromJson(Map<String, dynamic> json) =>
    GCMemberParted(
      json['gcid'] as String,
      json['uid'] as String,
      json['reason'] as String,
      json['kicked'] as bool? ?? false,
    );

Map<String, dynamic> _$GCMemberPartedToJson(GCMemberParted instance) =>
    <String, dynamic>{
      'gcid': instance.gcid,
      'uid': instance.uid,
      'reason': instance.reason,
      'kicked': instance.kicked,
    };

GCModifyAdmins _$GCModifyAdminsFromJson(Map<String, dynamic> json) =>
    GCModifyAdmins(
      json['gcid'] as String,
      (json['new_admins'] as List<dynamic>).map((e) => e as String).toList(),
    );

Map<String, dynamic> _$GCModifyAdminsToJson(GCModifyAdmins instance) =>
    <String, dynamic>{
      'gcid': instance.gcid,
      'new_admins': instance.newAdmins,
    };

GCAdminsChanged _$GCAdminsChangedFromJson(Map<String, dynamic> json) =>
    GCAdminsChanged(
      json['gcid'] as String,
      json['source'] as String,
      (json['added'] as List<dynamic>?)?.map((e) => e as String).toList(),
      (json['removed'] as List<dynamic>?)?.map((e) => e as String).toList(),
      json['changed_owner'] as bool,
    );

Map<String, dynamic> _$GCAdminsChangedToJson(GCAdminsChanged instance) =>
    <String, dynamic>{
      'gcid': instance.gcid,
      'source': instance.source,
      'added': instance.added,
      'removed': instance.removed,
      'changed_owner': instance.changedOwner,
    };

SubscribeToPosts _$SubscribeToPostsFromJson(Map<String, dynamic> json) =>
    SubscribeToPosts(
      json['target'] as String,
      json['fetch_post'] as String?,
    );

Map<String, dynamic> _$SubscribeToPostsToJson(SubscribeToPosts instance) {
  final val = <String, dynamic>{
    'target': instance.target,
  };

  void writeNotNull(String key, dynamic value) {
    if (value != null) {
      val[key] = value;
    }
  }

  writeNotNull('fetch_post', instance.fetchPost);
  return val;
}

RMKXSearchRef _$RMKXSearchRefFromJson(Map<String, dynamic> json) =>
    RMKXSearchRef(
      json['type'] as String,
      json['ref'] as String,
    );

Map<String, dynamic> _$RMKXSearchRefToJson(RMKXSearchRef instance) =>
    <String, dynamic>{
      'type': instance.type,
      'ref': instance.ref,
    };

RMKXSearch _$RMKXSearchFromJson(Map<String, dynamic> json) => RMKXSearch(
      (json['refs'] as List<dynamic>?)
              ?.map((e) => RMKXSearchRef.fromJson(e as Map<String, dynamic>))
              .toList() ??
          [],
    );

Map<String, dynamic> _$RMKXSearchToJson(RMKXSearch instance) =>
    <String, dynamic>{
      'refs': instance.refs,
    };

KXSearchQuery _$KXSearchQueryFromJson(Map<String, dynamic> json) =>
    KXSearchQuery(
      json['user'] as String,
      DateTime.parse(json['date_sent'] as String),
      (json['ids_received'] as List<dynamic>?)
              ?.map((e) => e as String)
              .toList() ??
          [],
    );

Map<String, dynamic> _$KXSearchQueryToJson(KXSearchQuery instance) =>
    <String, dynamic>{
      'user': instance.user,
      'date_sent': instance.dateSent.toIso8601String(),
      'ids_received': instance.idsReceived,
    };

KXSearch _$KXSearchFromJson(Map<String, dynamic> json) => KXSearch(
      json['target'] as String,
      RMKXSearch.fromJson(json['search'] as Map<String, dynamic>),
      (json['queries'] as List<dynamic>)
          .map((e) => KXSearchQuery.fromJson(e as Map<String, dynamic>))
          .toList(),
    );

Map<String, dynamic> _$KXSearchToJson(KXSearch instance) => <String, dynamic>{
      'target': instance.target,
      'search': instance.search,
      'queries': instance.queries,
    };

SuggestKX _$SuggestKXFromJson(Map<String, dynamic> json) => SuggestKX(
      json['invitee'] as String,
      json['target'] as String,
    );

Map<String, dynamic> _$SuggestKXToJson(SuggestKX instance) => <String, dynamic>{
      'invitee': instance.inviteeID,
      'target': instance.targetID,
    };

TransReset _$TransResetFromJson(Map<String, dynamic> json) => TransReset(
      json['mediator'] as String,
      json['target'] as String,
    );

Map<String, dynamic> _$TransResetToJson(TransReset instance) =>
    <String, dynamic>{
      'mediator': instance.mediatorID,
      'target': instance.targetID,
    };

KXSuggested _$KXSuggestedFromJson(Map<String, dynamic> json) => KXSuggested(
      json['alreadyknown'] as bool,
      json['inviteenick'] as String,
      json['invitee'] as String,
      json['targetnick'] as String,
      json['target'] as String,
    );

Map<String, dynamic> _$KXSuggestedToJson(KXSuggested instance) =>
    <String, dynamic>{
      'alreadyknown': instance.alreadyknown,
      'inviteenick': instance.inviteenick,
      'invitee': instance.invitee,
      'targetnick': instance.targetnick,
      'target': instance.target,
    };

TipProgressEvent _$TipProgressEventFromJson(Map<String, dynamic> json) =>
    TipProgressEvent(
      base64ToHex(json['uid'] as String),
      json['nick'] as String,
      json['attempt'] as int? ?? 0,
      json['completed'] as bool? ?? false,
      json['amount_matoms'] as int? ?? 0,
      json['attempt_err'] as String? ?? '',
      json['will_retry'] as bool? ?? false,
    );

Map<String, dynamic> _$TipProgressEventToJson(TipProgressEvent instance) =>
    <String, dynamic>{
      'uid': instance.uid,
      'nick': instance.nick,
      'attempt': instance.attempt,
      'completed': instance.completed,
      'amount_matoms': instance.amountMAtoms,
      'attempt_err': instance.attemptErr,
      'will_retry': instance.willRetry,
    };

Account _$AccountFromJson(Map<String, dynamic> json) => Account(
      json['name'] as String,
      json['unconfirmed_balance'] as int,
      json['confirmed_balance'] as int,
      json['internal_key_count'] as int,
      json['external_key_count'] as int,
    );

Map<String, dynamic> _$AccountToJson(Account instance) => <String, dynamic>{
      'name': instance.name,
      'unconfirmed_balance': instance.unconfirmedBalance,
      'confirmed_balance': instance.confirmedBalance,
      'internal_key_count': instance.internalKeyCount,
      'external_key_count': instance.externalKeyCount,
    };

LogEntry _$LogEntryFromJson(Map<String, dynamic> json) => LogEntry(
      json['from'] as String,
      json['message'] as String,
      json['internal'] as bool,
      json['timestamp'] as int,
    );

Map<String, dynamic> _$LogEntryToJson(LogEntry instance) => <String, dynamic>{
      'from': instance.from,
      'message': instance.message,
      'internal': instance.internal,
      'timestamp': instance.timestamp,
    };

SendOnChain _$SendOnChainFromJson(Map<String, dynamic> json) => SendOnChain(
      json['addr'] as String,
      json['amount'] as int,
      json['from_account'] as String,
    );

Map<String, dynamic> _$SendOnChainToJson(SendOnChain instance) =>
    <String, dynamic>{
      'addr': instance.addr,
      'amount': instance.amount,
      'from_account': instance.fromAccount,
    };

LoadUserHistory _$LoadUserHistoryFromJson(Map<String, dynamic> json) =>
    LoadUserHistory(
      json['uid'] as String,
      json['is_gc'] as bool,
      json['page'] as int,
      json['page_num'] as int,
    );

Map<String, dynamic> _$LoadUserHistoryToJson(LoadUserHistory instance) =>
    <String, dynamic>{
      'uid': instance.uid,
      'is_gc': instance.isGC,
      'page': instance.page,
      'page_num': instance.pageNum,
    };

WriteInvite _$WriteInviteFromJson(Map<String, dynamic> json) => WriteInvite(
      json['fund_amount'] as int,
      json['fund_account'] as String,
      json['gc_id'] as String?,
    );

Map<String, dynamic> _$WriteInviteToJson(WriteInvite instance) =>
    <String, dynamic>{
      'fund_amount': instance.fundAmount,
      'fund_account': instance.fundAccount,
      'gc_id': instance.gcid,
    };

RedeemedInviteFunds _$RedeemedInviteFundsFromJson(Map<String, dynamic> json) =>
    RedeemedInviteFunds(
      json['txid'] as String,
      json['total'] as int,
    );

Map<String, dynamic> _$RedeemedInviteFundsToJson(
        RedeemedInviteFunds instance) =>
    <String, dynamic>{
      'txid': instance.txid,
      'total': instance.total,
    };

OnboardState _$OnboardStateFromJson(Map<String, dynamic> json) => OnboardState(
      $enumDecode(_$OnboardStageEnumMap, json['stage']),
      json['key'] as String?,
      json['invite'] == null
          ? null
          : OOBPublicIdentityInvite.fromJson(
              json['invite'] as Map<String, dynamic>),
      dynListToHexReversed(json['redeem_tx'] as List?),
      json['redeem_amount'] as int? ?? 0,
      json['out_channel_id'] as String,
      json['in_channel_id'] as String,
      json['out_channel_height_hint'] as int? ?? 0,
      json['out_channel_mined_height'] as int? ?? 0,
      json['out_channel_confs_left'] as int? ?? 0,
    );

Map<String, dynamic> _$OnboardStateToJson(OnboardState instance) =>
    <String, dynamic>{
      'stage': _$OnboardStageEnumMap[instance.stage]!,
      'key': instance.key,
      'invite': instance.invite,
      'redeem_tx': instance.redeemTx,
      'redeem_amount': instance.redeemAmount,
      'out_channel_id': instance.outChannelID,
      'in_channel_id': instance.inChannelID,
      'out_channel_height_hint': instance.outChannelHeightHint,
      'out_channel_mined_height': instance.outChannelMinedHeight,
      'out_channel_confs_left': instance.outChannelConfsLeft,
    };

const _$OnboardStageEnumMap = {
  OnboardStage.stageFetchingInvite: 'fetching_invite',
  OnboardStage.stageInviteUnpaid: 'invite_unpaid',
  OnboardStage.stageInviteNoFunds: 'invite_no_funds',
  OnboardStage.stageInviteFetchTimeout: 'invite_fetch_timeout',
  OnboardStage.stageRedeemingFunds: 'redeeming_funds',
  OnboardStage.stageWaitingOutMined: 'waiting_out_mined',
  OnboardStage.stageWaitingFundsConfirm: 'waiting_funds_confirm',
  OnboardStage.stageOpeningOutbound: 'opening_outbound',
  OnboardStage.stageWaitingOutConfirm: 'waiting_out_confirm',
  OnboardStage.stageOpeningInbound: 'opening_inbound',
  OnboardStage.stageInitialKX: 'initial_kx',
  OnboardStage.stageOnboardDone: 'done',
};

FetchResourceArgs _$FetchResourceArgsFromJson(Map<String, dynamic> json) =>
    FetchResourceArgs(
      json['uid'] as String,
      (json['path'] as List<dynamic>).map((e) => e as String).toList(),
      (json['metadata'] as Map<String, dynamic>?)?.map(
        (k, e) => MapEntry(k, e as String),
      ),
      json['session_id'] as int? ?? 0,
      json['parent_page'] as int? ?? 0,
      json['data'],
      json['async_target_id'] as String,
    );

Map<String, dynamic> _$FetchResourceArgsToJson(FetchResourceArgs instance) =>
    <String, dynamic>{
      'uid': instance.uid,
      'path': instance.path,
      'metadata': instance.metadata,
      'session_id': instance.sessionID,
      'parent_page': instance.parentPage,
      'data': instance.data,
      'async_target_id': instance.asyncTargetID,
    };

RMFetchResource _$RMFetchResourceFromJson(Map<String, dynamic> json) =>
    RMFetchResource(
      (json['path'] as List<dynamic>).map((e) => e as String).toList(),
      (json['meta'] as Map<String, dynamic>?)?.map(
        (k, e) => MapEntry(k, e as String),
      ),
      hexToUint64(json['tag'] as String),
      json['data'],
      json['index'] as int,
      json['count'] as int,
    );

Map<String, dynamic> _$RMFetchResourceToJson(RMFetchResource instance) =>
    <String, dynamic>{
      'path': instance.path,
      'meta': instance.meta,
      'tag': instance.tag,
      'data': instance.data,
      'index': instance.index,
      'count': instance.count,
    };

RMFetchResourceReply _$RMFetchResourceReplyFromJson(
        Map<String, dynamic> json) =>
    RMFetchResourceReply(
      hexToUint64(json['tag'] as String),
      json['status'] as int,
      (json['meta'] as Map<String, dynamic>?)?.map(
        (k, e) => MapEntry(k, e as String),
      ),
      base64ToUint8list(json['data'] as String?),
      json['index'] as int,
      json['count'] as int,
    );

Map<String, dynamic> _$RMFetchResourceReplyToJson(
        RMFetchResourceReply instance) =>
    <String, dynamic>{
      'tag': instance.tag,
      'status': instance.status,
      'meta': instance.meta,
      'data': instance.data,
      'index': instance.index,
      'count': instance.count,
    };

SSProduct _$SSProductFromJson(Map<String, dynamic> json) => SSProduct(
      json['title'] as String,
      json['sku'] as String,
      json['description'] as String,
      (json['tags'] as List<dynamic>?)?.map((e) => e as String).toList() ?? [],
      (json['price'] as num).toDouble(),
      json['disabled'] as bool? ?? false,
    );

Map<String, dynamic> _$SSProductToJson(SSProduct instance) => <String, dynamic>{
      'title': instance.title,
      'sku': instance.sku,
      'description': instance.description,
      'tags': instance.tags,
      'price': instance.price,
      'disabled': instance.disabled,
    };

SSCartItem _$SSCartItemFromJson(Map<String, dynamic> json) => SSCartItem(
      SSProduct.fromJson(json['product'] as Map<String, dynamic>),
      json['quantity'] as int,
    );

Map<String, dynamic> _$SSCartItemToJson(SSCartItem instance) =>
    <String, dynamic>{
      'product': instance.product,
      'quantity': instance.quantity,
    };

SSCart _$SSCartFromJson(Map<String, dynamic> json) => SSCart(
      (json['items'] as List<dynamic>)
          .map((e) => SSCartItem.fromJson(e as Map<String, dynamic>))
          .toList(),
      DateTime.parse(json['updated'] as String),
    );

Map<String, dynamic> _$SSCartToJson(SSCart instance) => <String, dynamic>{
      'items': instance.items,
      'updated': instance.updated.toIso8601String(),
    };

SSOrder _$SSOrderFromJson(Map<String, dynamic> json) => SSOrder(
      json['id'] as int,
      json['user'] as String,
      SSCart.fromJson(json['cart'] as Map<String, dynamic>),
    );

Map<String, dynamic> _$SSOrderToJson(SSOrder instance) => <String, dynamic>{
      'id': instance.id,
      'user': instance.user,
      'cart': instance.cart,
    };

SSPlacedOrder _$SSPlacedOrderFromJson(Map<String, dynamic> json) =>
    SSPlacedOrder(
      SSOrder.fromJson(json['order'] as Map<String, dynamic>),
      json['msg'] as String,
    );

Map<String, dynamic> _$SSPlacedOrderToJson(SSPlacedOrder instance) =>
    <String, dynamic>{
      'order': instance.order,
      'msg': instance.msg,
    };

FetchedResource _$FetchedResourceFromJson(Map<String, dynamic> json) =>
    FetchedResource(
      json['uid'] as String,
      json['session_id'] as int,
      json['parent_page'] as int,
      json['page_id'] as int,
      DateTime.parse(json['request_ts'] as String),
      DateTime.parse(json['response_ts'] as String),
      RMFetchResource.fromJson(json['request'] as Map<String, dynamic>),
      RMFetchResourceReply.fromJson(json['response'] as Map<String, dynamic>),
      json['async_target_id'] as String,
    );

Map<String, dynamic> _$FetchedResourceToJson(FetchedResource instance) =>
    <String, dynamic>{
      'uid': instance.uid,
      'session_id': instance.sessionID,
      'parent_page': instance.parentPage,
      'page_id': instance.pageID,
      'request_ts': instance.requestTS.toIso8601String(),
      'response_ts': instance.responseTS.toIso8601String(),
      'request': instance.request,
      'response': instance.response,
      'async_target_id': instance.asyncTargetID,
    };

LoadFetchedResourceArgs _$LoadFetchedResourceArgsFromJson(
        Map<String, dynamic> json) =>
    LoadFetchedResourceArgs(
      json['uid'] as String,
      json['session_id'] as int,
      json['page_id'] as int,
    );

Map<String, dynamic> _$LoadFetchedResourceArgsToJson(
        LoadFetchedResourceArgs instance) =>
    <String, dynamic>{
      'uid': instance.uid,
      'session_id': instance.sessionID,
      'page_id': instance.pageID,
    };

HandshakeStage _$HandshakeStageFromJson(Map<String, dynamic> json) =>
    HandshakeStage(
      json['uid'] as String,
      json['stage'] as String,
    );

Map<String, dynamic> _$HandshakeStageToJson(HandshakeStage instance) =>
    <String, dynamic>{
      'uid': instance.uid,
      'stage': instance.stage,
    };

ListTransactionsArgs _$ListTransactionsArgsFromJson(
        Map<String, dynamic> json) =>
    ListTransactionsArgs(
      json['start_height'] as int,
      json['end_height'] as int,
    );

Map<String, dynamic> _$ListTransactionsArgsToJson(
        ListTransactionsArgs instance) =>
    <String, dynamic>{
      'start_height': instance.startHeight,
      'end_height': instance.endHeight,
    };

Transaction _$TransactionFromJson(Map<String, dynamic> json) => Transaction(
      json['tx_hash'] as String,
      json['amount'] as int,
      json['block_height'] as int,
    );

Map<String, dynamic> _$TransactionToJson(Transaction instance) =>
    <String, dynamic>{
      'tx_hash': instance.txHash,
      'amount': instance.amount,
      'block_height': instance.blockHeight,
    };

PostAndCommandID _$PostAndCommandIDFromJson(Map<String, dynamic> json) =>
    PostAndCommandID(
      json['post_id'] as String,
      json['comment_id'] as String,
    );

Map<String, dynamic> _$PostAndCommandIDToJson(PostAndCommandID instance) =>
    <String, dynamic>{
      'post_id': instance.postID,
      'comment_id': instance.commentID,
    };

ReceiveReceipt _$ReceiveReceiptFromJson(Map<String, dynamic> json) =>
    ReceiveReceipt(
      json['user'] as String,
      json['server_time'] as int,
      json['client_time'] as int,
    );

Map<String, dynamic> _$ReceiveReceiptToJson(ReceiveReceipt instance) =>
    <String, dynamic>{
      'user': instance.user,
      'server_time': instance.serverTime,
      'client_time': instance.clientTime,
    };

ProfileUpdated _$ProfileUpdatedFromJson(Map<String, dynamic> json) =>
    ProfileUpdated(
      json['sid'] as String,
      AddressBookEntry.fromJson(
          json['addressbook_entry'] as Map<String, dynamic>),
      (json['updated_fields'] as List<dynamic>)
          .map((e) => e as String)
          .toList(),
    );

Map<String, dynamic> _$ProfileUpdatedToJson(ProfileUpdated instance) =>
    <String, dynamic>{
      'sid': instance.uid,
      'addressbook_entry': instance.abEntry,
      'updated_fields': instance.updatedFields,
    };

RunState _$RunStateFromJson(Map<String, dynamic> json) => RunState(
      dcrlndRunning: json['dcrlnd_running'] as bool,
      clientRunning: json['client_running'] as bool,
    );

Map<String, dynamic> _$RunStateToJson(RunState instance) => <String, dynamic>{
      'dcrlnd_running': instance.dcrlndRunning,
      'client_running': instance.clientRunning,
    };

ZipLogsArgs _$ZipLogsArgsFromJson(Map<String, dynamic> json) => ZipLogsArgs(
      json['include_golib'] as bool,
      json['include_ln'] as bool,
      json['only_last_file'] as bool,
      json['dest_path'] as String,
    );

Map<String, dynamic> _$ZipLogsArgsToJson(ZipLogsArgs instance) =>
    <String, dynamic>{
      'include_golib': instance.includeGolib,
      'include_ln': instance.includeLn,
      'only_last_file': instance.onlyLastFile,
      'dest_path': instance.destPath,
    };
