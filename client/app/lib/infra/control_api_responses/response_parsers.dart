/// Flutter 侧控制面响应解析入口。
library slan_app.infra.control_api_responses.response_parsers;

import '../app_core/models/bootstrap_models.dart';
import '../app_core/models/identity_models.dart';
import '../app_core/models/network_models.dart';
import '../app_core/models/relay_models.dart';
import 'json_readers.dart';
import 'response_mappers.dart';
import 'response_dtos.dart';

SessionModel parseSessionResponse(Map<String, dynamic> json) =>
    AuthResponseDto.fromJson(json).toModel();

DeviceModel parseDeviceResponse(Map<String, dynamic> json) =>
    DeviceResponseDto.fromJson(json).toModel();

List<DeviceModel> parseDeviceListResponse(List<dynamic> json) => json
    .map((item) => DeviceResponseDto.fromJson(asMap(item)).toModel())
    .toList(growable: false);

NodeModel parseNodeResponse(Map<String, dynamic> json) =>
    NodeResponseDto.fromJson(json).toModel();

List<NetworkModel> parseNetworkListResponse(List<dynamic> json) => json
    .map((item) => NetworkSummaryResponseDto.fromJson(asMap(item)).toModel())
    .toList(growable: false);

NetworkModel parseNetworkResponse(Map<String, dynamic> json) =>
    NetworkSummaryResponseDto.fromJson(json).toModel();

NetworkJoinModel parseNetworkJoinResponse(Map<String, dynamic> json) =>
    NetworkJoinResultResponseDto.fromJson(json).toModel();

NetworkJoinModel parseNetworkJoinByOwnerEmailResponse(
  Map<String, dynamic> json,
) =>
    NetworkJoinByOwnerEmailResultResponseDto.fromJson(json).toModel();

NetworkAssignmentModel parseNetworkAssignmentResponse(
  Map<String, dynamic> json,
) =>
    NetworkAssignmentResponseDto.fromJson(json).toModel();

BootstrapModel parseBootstrapResponse(Map<String, dynamic> json) =>
    BootstrapResponseDto.fromJson(json).toModel();

RelayTicketModel parseRelayTicketResponse(Map<String, dynamic> json) =>
    RelayTicketResponseDto.fromJson(json).toModel();
