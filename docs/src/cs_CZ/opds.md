# Katalog e-knih

Readeck poskytuje Katalog e-knih všech vašich záložek uspořádaných do katalogu s níže uvedenou strukturou:

- Nepřečtené záložky
- Archivované záložky
- Oblíbené záložky
- Všechny záložky
- Sbírky záložek
  - (Název sbírky)
    - E-kniha sbírky
    - Procházet sbírku

V každé sekci, kromě Sbírek, je každá záložka k dispozici jako e-kniha.

V sekci sbírky si můžete stáhnout celou sbírku jako jednu e-knihu.


## Přístup ke katalogu

Ke katalogu lze přistupovat pomocí jakékoliv aplikace nebo elektronické čtečky podporující formát OPDS.
Pro poskytnutí přístupu ke katalogu musíte nejprve vytvořit [Heslo aplikace](readeck-instance://profile/credentials).

Oprávnění tohoto hesla můžete omezit na „Záložky : Pouze pro čtení“.
Zapište si heslo a nastavte aplikaci.

Adresa URL vašeho katalogu OPDS je následující: \
[readeck-instance://opds](readeck-instance://opds)


## Příklad nastavení: KOReader

[KOReader](https://koreader.rocks/) je prohlížeč dokumentů pro zařízení s elektronickým inkoustrem. Je dostupný pro Kindle, Kobo, PocketBook, Android a desktopový Linux. Má velmi dobrou podporu OPDS.

Po zadání Hesla aplikace můžete přistupovat k sekci OPDS v nabídce vyhledávání KOReaderu:

[Nabídka vyhledávání v KOReaderu](../img/koreader-1.webp).

Tato sekce zobrazuje seznam předkonfigurovaných zdrojů OPDS a svůj můžete přidat stisknutím ikony „+“ v levém horním rohu:

![Seznam katalogů v KOReaderu](../img/koreader-2.webp)

V tomto dialogovém okně nahraďte níže uvedená pole následujícími údaji:

- https://readeck.example.com : `readeck-instance://opds`
- alice: vaše uživatelské jméno
- vaše heslo aplikace v posledním poli

![Přidání katalogu v KOReaderu](../img/koreader-3.webp)

Nyní můžete přistupovat ke svým záložkám z KOReaderu!

![Katalog Readecku v KORederu](../img/koreader-4.webp)