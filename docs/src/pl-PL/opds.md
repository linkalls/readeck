# Katalog e-booków

Readeck udostępnia Katalog E-booków ze wszystkimi twoimi zakładkami, zorganizowany w katalog ze strukturą jak poniżej:

- Nieprzeczytane zakładki
- Zarchiwizowane zakładki
- Ulubione zakładki
- Wszystkie zakładki
- Kolekcje zakładek
  - (Nazwa kolekcji)
    - E-book kolekcji
    - Przeglądaj kolekcję

Każda sekcja, z wyjątkiem Kolekcji, zawiera każdą zakładkę jako e-book.

W sekcji kolekcji możesz ściągnąć całą kolekcję jako jeden e-book.

## Dostęp do katalogu

Do katalogu można uzyskać dostęp z każdej aplikacji lub czytnika obsługującego format OPDS.
Aby przydzielić dostęp do katalogu musisz najpierw stworzyć [Hasło Aplikacji](readeck-instance://profile/credentials).

Możesz ograniczyć prawa dostępu hasła tylko do "Zakładki : Tylko odczyt".
Zapisz sobie hasło i skonfiguruj aplikację.

Adres URL twojego katalogu OPDS to: \
[readeck-instance://opds](readeck-instance://opds)

## Przykładowa konfiguracja: Koreader

[Koreader](https://koreader.rocks/) jest przeglądarką dokumentów dla urządzeń z ekranem E-Ink. Jest ona dostępna dla Kindle, Kobo, PocketBook'a, Android'a i Linux's. Posiada bardzo dobre wsparcie OPDS.

Gdy będziesz miał Hasło Aplikacji możesz uzyskać dostęp do sekcji OPDS w menu wyszukiwanie Koreader:

![Menu wyszukiwania Koreader](../img/koreader-1.webp)

Sekcja ta pokazuje listę pre-konfigurowanych źródeł OPDS i możesz tutaj dodać swoje źródło naciskając ikonę "+" w lewym górnym rogu:

![Lista katalogów Koreader](../img/koreader-2.webp)

W tym oknie dialogowym zamień pola poniżej na:

- [https://readeck.example.com](https://readeck.example.com) : `readeck-instance://opds`
- alice: twoja nazwa użytkownika
- twoje hasło aplikacji w ostatnim polu

![Dodaj katalog Koreader](../img/koreader-3.webp)

Teraz możesz uzyskać dostęp do swoich zakładek z Koreader'a!

![Katalo readeck oreader](../img/koreader-4.webp)
