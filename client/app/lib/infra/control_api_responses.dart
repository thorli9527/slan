/// Flutter 侧控制面响应 DTO 与 AppCore 模型映射。
///
/// 这层负责把真实控制面 JSON 解析为页面层当前使用的模型，
/// 避免 UI 继续依赖 demo/mock 手工拼装数据。
library slan_app.infra.control_api_responses;

import 'app_core/models/models.dart';
import 'control_api_responses/json_readers.dart';
import 'control_api_responses/response_dtos.dart';

SessionModel parseSessionResponse(Map<String, dynamic> json) =>
    AuthResponseDto.fromJson(json).toModel();

DeviceModel parseDeviceResponse(Map<String, dynamic> json) =>
    DeviceResponseDto.fromJson(json).toModel();

NodeModel parseNodeResponse(Map<String, dynamic> json) =>
    NodeResponseDto.fromJson(json).toModel();

List<NetworkModel> parseNetworkListResponse(List<dynamic> json) => json
    .map((item) => NetworkSummaryResponseDto.fromJson(asMap(item)).toModel())
    .toList(growable: false);

NetworkModel parseNetworkResponse(Map<String, dynamic> json) =>
    NetworkSummaryResponseDto.fromJson(json).toModel();

BootstrapModel parseBootstrapResponse(Map<String, dynamic> json) =>
    BootstrapResponseDto.fromJson(json).toModel();

RelayTicketModel parseRelayTicketResponse(Map<String, dynamic> json) =>
    RelayTicketResponseDto.fromJson(json).toModel();
