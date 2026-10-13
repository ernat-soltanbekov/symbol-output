# Источники шрифтов

Все три таблицы взяты из официального репозитория `01-edu/public`, каталога
`subjects/ascii-art`. Их содержимое сохранено без изменений, включая конечные
пробелы. У `thinkertoy.txt` оригинальные разделители CRLF; загрузчик нормализует
их в LF, а выходной рисунок всегда использует LF.

| Файл | Официальный источник | SHA-256 исходных байтов |
| --- | --- | --- |
| standard.txt | [01-edu/public](https://raw.githubusercontent.com/01-edu/public/master/subjects/ascii-art/standard.txt) | `e194f1033442617ab8a78e1ca63a2061f5cc07a3f05ac226ed32eb9dfd22a6bf` |
| shadow.txt | [01-edu/public](https://raw.githubusercontent.com/01-edu/public/master/subjects/ascii-art/shadow.txt) | `26b94d0b134b77e9fd23e0360bfd81740f80fb7f6541d1d8c5d85e73ee550f73` |
| thinkertoy.txt | [01-edu/public](https://raw.githubusercontent.com/01-edu/public/master/subjects/ascii-art/thinkertoy.txt) | `64285e4960d199f4819323c4dc6319ba34f1f0dd9da14d07111345f5d76c3fa3` |

Каждая таблица содержит 95 печатных ASCII-символов, от кода 32 до 126.
Для каждого: пустая строка-разделитель и 8 строк рисунка одинаковой ширины.
Всего 855 строк, плюс завершающий перевод строки. Ширина разных символов различна.

Корневой тест проверяет SHA-256 встроенных данных, чтобы незаметное изменение
одного пробела не испортило эталонные рисунки. `.gitattributes` запрещает Git
автоматически менять окончания строк в этих файлах.
