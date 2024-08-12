# 个人信息

Readeck [个人信息](readeck-instance://profile) 允许您更改个人信息、密码和其他一些设置。

## 编辑个人信息

在个人信息页面上，您可以更改用户名、电子邮件地址并选择应用程序的语言。

## 修改密码

在 [密码](readeck-instance://profile/password) 页面, 您可以更改 Readeck 的密码。

## API Tokens

API Token 允许您访问和使用 [Readeck API](readeck-instance://docs/api) 任何你想建造的东西。 你可以在 [API Tokens](readeck-instance://profile/tokens) 上创建和管理 API Tokens。

您可以限制给定令牌可以通过API访问的内容以及其有效期。

## 应用密码

如果您需要将Readeck帐户的访问权限授予服务或应用程序，则无法提供主用户名和密码；这行不通。

你可以创建一个 [应用密码](readeck-instance://profile/credentials).

您可以通过 API 限制给定密码可以访问的内容。

创建应用程序密码后，可以使用该密码访问 [Readeck API](readeck-instance://docs/api) 或导出服务。

查看 [电子书分类](./opds.md) 一个真实示例的帮助页面。

**注意**: 虽然您可以使用应用程序密码访问API，但建议尽可能使用 API Token。
