# OPDS

Readeck 提供了一个包含您所有书签的OPDS，按以下结构组织在目录中：

- 未读书签
- 归档书签
- 收藏书签
- 所有书签
- 书签集合
  - (集合名称)
    - 搜集电子书
    - 选择集合

除集合外，每个部分都将每个书签作为电子书提供。

在集合部分，您可以将完整的集合作为一本电子书下载。


## 目录访问

任何支持OPDS格式的应用程序或电子阅读器都可以访问该目录。
要授予对目录的访问权限，您必须首先创建 [应用密码](readeck-instance://profile/credentials).

您可以将此密码的权限限制为“书签：只读”。
记下密码并设置应用程序。

你的OPDS的URL 是: [readeck-instance://opds](readeck-instance://opds)


## 示例设置: Koreader

[Koreader](https://koreader.rocks/) 是 E Ink设备的文档查看器。它适用于Kindle、Kobo、PocketBook、Android和桌面Linux。它有很好的OPDS支持。

一旦您有了应用程序密码，您就可以访问Koreander查找菜单中的OPDS部分：

![Koreader's lookup menu](../img/koreader-1.webp)

本节显示了预配置的OPDS源列表，您可以通过按左上角的“+”图标添加您的源：

![Koreader catalog list](../img/koreader-2.webp)

在此对话框中，将以下字段替换为：

- https://readeck.example.com : `readeck-instance://opds`
- alice: 你的用户名
- 最后一个字段中的应用程序密码

![Koreader add catalog](../img/koreader-3.webp)

你现在可以从 Koreader 访问你的书签!

![Koreader readeck catalog](../img/koreader-4.webp)
