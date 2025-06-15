import 'package:freezed_annotation/freezed_annotation.dart';

part 'bookmark.freezed.dart';
part 'bookmark.g.dart';

@freezed
class BookmarkResource with _$BookmarkResource {
  const factory BookmarkResource({
    String? src,
  }) = _BookmarkResource;

  factory BookmarkResource.fromJson(Map<String, dynamic> json) =>
      _$BookmarkResourceFromJson(json);
}

@freezed
class BookmarkResourceImage with _$BookmarkResourceImage {
  const factory BookmarkResourceImage({
    String? src,
    int? height,
    int? width,
  }) = _BookmarkResourceImage;

  factory BookmarkResourceImage.fromJson(Map<String, dynamic> json) =>
      _$BookmarkResourceImageFromJson(json);
}

@freezed
class BookmarkResources with _$BookmarkResources {
  const factory BookmarkResources({
    BookmarkResourceImage? icon,
    BookmarkResourceImage? image,
    BookmarkResourceImage? thumbnail,
    BookmarkResource? article,
    BookmarkResource? log,
    BookmarkResource? props,
  }) = _BookmarkResources;

  factory BookmarkResources.fromJson(Map<String, dynamic> json) =>
      _$BookmarkResourcesFromJson(json);
}

@freezed
class BookmarkSummary with _$BookmarkSummary {
  const factory BookmarkSummary({
    String? id,
    String? href,
    DateTime? created,
    DateTime? updated,
    int? state,
    bool? loaded,
    String? url,
    String? title,
    @JsonKey(name: 'site_name') String? siteName,
    String? site,
    DateTime? published,
    List<String>? authors,
    String? lang,
    @JsonKey(name: 'text_direction') String? textDirection,
    @JsonKey(name: 'document_type') String? documentType,
    String? type,
    @JsonKey(name: 'has_article') bool? hasArticle,
    String? description,
    @JsonKey(name: 'is_deleted') bool? isDeleted,
    @JsonKey(name: 'is_marked') bool? isMarked,
    @JsonKey(name: 'is_archived') bool? isArchived,
    @JsonKey(name: 'read_progress') int? readProgress,
    List<String>? labels,
    @JsonKey(name: 'word_count') int? wordCount,
    @JsonKey(name: 'reading_time') int? readingTime,
    BookmarkResources? resources,
  }) = _BookmarkSummary;

  factory BookmarkSummary.fromJson(Map<String, dynamic> json) =>
      _$BookmarkSummaryFromJson(json);
}

@freezed
class BookmarkLink with _$BookmarkLink {
  const factory BookmarkLink({
    String? url,
    String? domain,
    String? title,
    @JsonKey(name: 'is_page') bool? isPage,
    @JsonKey(name: 'content_type') String? contentType,
  }) = _BookmarkLink;

  factory BookmarkLink.fromJson(Map<String, dynamic> json) =>
      _$BookmarkLinkFromJson(json);
}

@freezed
class BookmarkInfo with _$BookmarkInfo {
  const factory BookmarkInfo({
    String? id,
    String? href,
    DateTime? created,
    DateTime? updated,
    int? state,
    bool? loaded,
    String? url,
    String? title,
    @JsonKey(name: 'site_name') String? siteName,
    String? site,
    DateTime? published,
    List<String>? authors,
    String? lang,
    @JsonKey(name: 'text_direction') String? textDirection,
    @JsonKey(name: 'document_type') String? documentType,
    String? type,
    @JsonKey(name: 'has_article') bool? hasArticle,
    String? description,
    @JsonKey(name: 'is_deleted') bool? isDeleted,
    @JsonKey(name: 'is_marked') bool? isMarked,
    @JsonKey(name: 'is_archived') bool? isArchived,
    @JsonKey(name: 'read_progress') int? readProgress,
    List<String>? labels,
    @JsonKey(name: 'word_count') int? wordCount,
    @JsonKey(name: 'reading_time') int? readingTime,
    BookmarkResources? resources,
    @JsonKey(name: 'omit_description') bool? omitDescription,
    @JsonKey(name: 'read_anchor') String? readAnchor,
    List<BookmarkLink>? links,
  }) = _BookmarkInfo;

  factory BookmarkInfo.fromJson(Map<String, dynamic> json) =>
      _$BookmarkInfoFromJson(json);
}

@freezed
class BookmarkCreate with _$BookmarkCreate {
  const factory BookmarkCreate({
    required String url,
    String? title,
    List<String>? labels,
  }) = _BookmarkCreate;

  factory BookmarkCreate.fromJson(Map<String, dynamic> json) =>
      _$BookmarkCreateFromJson(json);
}

@freezed
class BookmarkUpdate with _$BookmarkUpdate {
  const factory BookmarkUpdate({
    String? title,
    @JsonKey(name: 'is_marked') bool? isMarked,
    @JsonKey(name: 'is_archived') bool? isArchived,
    @JsonKey(name: 'is_deleted') bool? isDeleted,
    @JsonKey(name: 'read_progress') int? readProgress,
    @JsonKey(name: 'read_anchor') String? readAnchor,
    List<String>? labels,
    @JsonKey(name: 'add_labels') List<String>? addLabels,
    @JsonKey(name: 'remove_labels') List<String>? removeLabels,
  }) = _BookmarkUpdate;

  factory BookmarkUpdate.fromJson(Map<String, dynamic> json) =>
      _$BookmarkUpdateFromJson(json);
}

@freezed
class BookmarkUpdated with _$BookmarkUpdated {
  const factory BookmarkUpdated({
    String? href,
    String? id,
    DateTime? updated,
    String? title,
    @JsonKey(name: 'is_marked') bool? isMarked,
    @JsonKey(name: 'is_archived') bool? isArchived,
    @JsonKey(name: 'is_deleted') bool? isDeleted,
    @JsonKey(name: 'read_progress') int? readProgress,
    @JsonKey(name: 'read_anchor') String? readAnchor,
    List<String>? labels,
  }) = _BookmarkUpdated;

  factory BookmarkUpdated.fromJson(Map<String, dynamic> json) =>
      _$BookmarkUpdatedFromJson(json);
}

@freezed
class BookmarkSync with _$BookmarkSync {
  const factory BookmarkSync({
    String? id,
    String? href,
    DateTime? created,
    DateTime? updated,
  }) = _BookmarkSync;

  factory BookmarkSync.fromJson(Map<String, dynamic> json) =>
      _$BookmarkSyncFromJson(json);
}

@freezed
class BookmarkShareLink with _$BookmarkShareLink {
  const factory BookmarkShareLink({
      String? url,
      DateTime? expires,
      String? title,
      String? id,
  }) = _BookmarkShareLink;

  factory BookmarkShareLink.fromJson(Map<String, dynamic> json) => _$BookmarkShareLinkFromJson(json);
}

@freezed
class BookmarkShareEmail with _$BookmarkShareEmail {
  const factory BookmarkShareEmail({
    required String email,
    required String format,
  }) = _BookmarkShareEmail;

  factory BookmarkShareEmail.fromJson(Map<String, dynamic> json) => _$BookmarkShareEmailFromJson(json);
}
