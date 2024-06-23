# Profil uživatele

V [sekci Profil](readeck-instance://profile) Readecku můžete změnit své osobní údaje, heslo a některá další nastavení.

## Úprava profilu

Na hlavní stránce profilu můžete změnit své uživatelské jméno, e-mailovou adresu a zvolit jazyk aplikace.

## Změna hesla

Na stránce [Heslo](readeck-instance://profile/password) můžete změnit heslo, které používáte pro připojení k Readecku.

## Tokeny API

Token API vám umožní přístup k rozhraní [API Readecku](readeck-instance://docs/api) a jeho použití pro cokoliv, co chcete vytvořit. Tokeny můžete vytvářet a spravovat v sekci [Tokeny API](readeck-instance://profile/tokens) svého uživatelského profilu.

Prostřednictvím rozhraní API můžete omezit, k čemu má daný token přístup a jak dlouho je platný.

## Hesla aplikací

Pokud potřebujete udělit přístup ke svému účtu Readeck nějaké službě nebo aplikaci, nemůžete poskytnout své hlavní uživatelské jméno a heslo; nebude to fungovat.

Můžete však vytvořit [Heslo aplikace](readeck-instance://profile/credentials).

Můžete omezit, k čemu má dané heslo přístup prostřednictvím rozhraní API.

Jakmile vytvoříte heslo aplikace, můžete ho použít pro přístup k [API Readecku](readeck-instance://docs/api) nebo službám exportu.

Reálný příklad naleznete na stránce nápovědy [Katalog e-knih](./opds.md).

**Poznámka**: Ačkoliv můžete pro přístup k rozhraní API použít heslo aplikace, doporučuje se používat token API, pokud je to možné.