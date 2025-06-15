import 'package:freezed_annotation/freezed_annotation.dart';

part 'collection.freezed.dart';
part 'collection.g.dart';

@freezed
class CollectionSummary with _$CollectionSummary {
  const factory CollectionSummary({
    DateTime? updated,
    String? name,
    @JsonKey(name: 'is_pinned') bool? isPinned,
    @JsonKey(name: 'is_deleted') bool? isDeleted,
    String? search,
    String? title,
    String? author,
    String? site,
    List<String>? type,
    List<String>? labels,
    @JsonKey(name: 'read_status') List<String>? readStatus,
    @JsonKey(name: 'is_marked') bool? isMarked,
    @JsonKey(name: 'is_archived') bool? isArchived,
    @JsonKey(name: 'range_start') String? rangeStart,
    @JsonKey(name: 'range_end') String? rangeEnd,
  }) = _CollectionSummary;

  factory CollectionSummary.fromJson(Map<String, dynamic> json) =>
      _$CollectionSummaryFromJson(json);
}

@freezed
class CollectionInfo with _$CollectionInfo {
  const factory CollectionInfo({
    DateTime? updated,
    String? name,
    @JsonKey(name: 'is_pinned') bool? isPinned,
    @JsonKey(name: 'is_deleted') bool? isDeleted,
    String? search,
    String? title,
    String? author,
    String? site,
    List<String>? type,
    List<String>? labels,
    @JsonKey(name: 'read_status') List<String>? readStatus,
    @JsonKey(name: 'is_marked') bool? isMarked,
    @JsonKey(name: 'is_archived') bool? isArchived,
    @JsonKey(name: 'range_start') String? rangeStart,
    @JsonKey(name: 'range_end') String? rangeEnd,
    String? id,
    String? href,
    DateTime? created,
  }) = _CollectionInfo;

  factory CollectionInfo.fromJson(Map<String, dynamic> json) =>
      _$CollectionInfoFromJson(json);
}

@freezed
class CollectionCreateOrUpdate with _$CollectionCreateOrUpdate {
  const factory CollectionCreateOrUpdate({
    String? name,
    @JsonKey(name: 'is_pinned') bool? isPinned,
    @JsonKey(name: 'is_deleted') bool? isDeleted,
    String? search,
    String? title,
    String? author,
    String? site,
    List<String>? type,
    List<String>? labels,
    @JsonKey(name: 'read_status') List<String>? readStatus,
    @JsonKey(name: 'is_marked') bool? isMarked,
    @JsonKey(name: 'is_archived') bool? isArchived,
    @JsonKey(name: 'range_start') String? rangeStart,
    @JsonKey(name: 'range_end') String? rangeEnd,
  }) = _CollectionCreateOrUpdate;

  factory CollectionCreateOrUpdate.fromJson(Map<String, dynamic> json) =>
      _$CollectionCreateOrUpdateFromJson(json);
}
