# Матрица требований и воспроизводимый аудит

Этот файл сопоставляет пункты предоставленного ТЗ с проверками проекта. Команды ниже — инструкция для повторной проверки, а не утверждение, что каждая команда уже выполнена. Результаты реально проведённых проверок записываются отдельно.

## Как трактуются спорные места ТЗ

1. В нескольких примерах `cat -e` после восьми строк рисунка показан отдельный `$`, будто у файла есть девятая пустая строка. Однако обязательный пример проверки прямо требует для `hello` **8 строк, ширину 31 и размер 256 байт**. Проект следует этому измеримому требованию: `8 × 31 + 8 = 256`, без дополнительной пустой строки. Для `First\nTest` получаются 16 строк.
2. Настоящий перенос LF и буквальная пара `\n` обозначают границу текстовых строк. В непустом вводе каждая строка, включая пустую внутри или в конце, занимает восемь строк рисунка. Поэтому `A\n` даёт 16 строк, `A\n\nB` — 24, а `\n` — 16 пустых строк. Полностью пустой аргумент `''` является исключением: это отсутствие текста, результат — пустой файл, 0 текстовых строк и 0 байт. Так сохраняется поведение базового `symbol-art`; это исключение явно документировано и проверяется тестами.
3. Пробелы справа входят в рисунок. Их удаление изменяет ширину и разрушает прямоугольник. Проверка padding запрещает выход за **ожидаемую ширину** данной текстовой строки, а не любые завершающие пробелы.
4. Структурная проверка `--verify` проверяет пять свойств файла. Она не доказывает совпадение каждого символа с эталоном. Замена символа без изменения размера может сохранить `5/5`; для сравнения содержания предназначен `--tamper-check`.
5. Существующий файл при `--tamper-check` нельзя сначала перезаписать: тогда следы изменения исчезнут. Проект сравнивает существующий файл с новой генерацией в памяти, сохраняя исходные байты. Если файла ещё нет, проверка сообщает ошибку: сначала результат нужно создать обычным `--output`. Обычный `--output` обновляет существующий результат. Совместный вызов `--verify --tamper-check` тоже читает существующий файл, не перезаписывая его.
6. В ТЗ нет реализованных опций других проектов семейства. Здесь поддерживаются `--output`, `--verify` и бонусный `--tamper-check`. Неизвестные опции отклоняются; расширение проекта не имитируется неработающими флагами.

## Соответствие всем пунктам аудита

| Требование | Проверяемое поведение | Где и чем проверять |
|---|---|---|
| Go и стандартная библиотека | Сборка без внешних модулей | `go.mod`, `go list -m all`, `go build ./...` |
| Структура сдачи | `main.go`, `go.mod`, три `.txt`-шрифта; пакеты `internal/generator`, `internal/writer`, `internal/verifier` и их тесты | Дерево файлов, `go test ./...` |
| README | Цель, запись результата, установка, структура, примеры `--verify` и бонуса на русском | `README.md` |
| Одно значение STRING | `hello` выводится в stdout шрифтом `standard` | Команда 2 ниже; сравнение с `--output` |
| Строгое `--output=<fileName.txt>` | Раздельное `--output test00.txt` отвергается | Команда 3; тесты разбора аргументов |
| Точное Usage | Две строки из ТЗ при неправильном формате аргументов | Команда 3, сравнение текста, ненулевой код завершения |
| Выбор шрифта | `standard`, `shadow`, `thinkertoy`; неизвестный шрифт отклоняется | Команды 4–6; тесты генератора |
| `First\nTest` с `shadow` | Два текстовых ряда, 16 линий, ширина каждого ряда проверяется отдельно | Команда 4 |
| `hello` с `standard` | Восемь линий по 31 байту, файл 256 байт | Команда 5, тест известного результата |
| `123 -> #$%` с `standard` | Цифры, пробелы, стрелка и специальные символы не теряются | Команда 6; этот пункт в приложенном аудите повторён дважды |
| Случайный смешанный регистр, цифры и пробелы | Создаётся указанный `.txt`-файл; отчёт `5/5` | Команда 7 |
| Разные специальные символы | Все печатные ASCII от 32 до 126 поддерживаются | Команда 8; тесты всех символов во всех шрифтах |
| Правильное имя/расположение результата | Путь из `--output` используется как путь файла | Команды 4–8 создают файлы в отдельном временном каталоге |
| Нет `--verify` | В файле только рисунок; обычный stdout не содержит отчёта проверки | Команда 5 без `--verify`, тесты CLI |
| Записать, затем перечитать | После успешной записи `--verify` читает файл через `os.ReadFile` | Код `internal/writer` и `internal/verifier`; тесты с повреждёнными файлами |
| Readable | Проверка фактически прочитанного файла; ошибка чтения не выдаётся за успех | Тесты `internal/verifier`, ошибка отсутствующего файла |
| Line count | `8 × число текстовых строк`, с описанным выше правилом пустого ввода | Команда 9; тесты отсутствующей/добавленной линии |
| Consistent width | Внутри каждого блока из восьми строк ширина одинакова; соседние блоки могут различаться | Команда 4; тесты повреждённой ширины |
| Padding | Нормальные пробелы сохранены; лишние колонки за шириной ряда отвергаются | Тесты лишнего пробела в одной и во всех строках |
| Size | Байтовый размер сопоставлен с суммой ширин строк и LF; каждый LF занимает один байт | Команда 5; тесты потерянного последнего LF и изменённого размера |
| Score/verdict | Пять успехов → `Valid`, четыре → `Acceptable`, 0–3 → `Corrupted` | Модульные тесты подсчёта и отчёта |
| Обязательный пример проверки | `Integrity Score: 5/5 (Valid)`, line count 8, width 31, size 256 | Команда 5 с `--verify` |
| Файлы разных шрифтов | Созданы воспроизводимые примеры `standard`, `shadow`, `thinkertoy` | Команды 4–8, файлы примеров проекта |
| Исходные шрифты | Все 95 печатных символов имеют восемь строк и правильные разделители | `standard.txt`, `shadow.txt`, `thinkertoy.txt`; тесты `generator.Parse` |
| Понятный и модульный код | Разбор CLI, загрузка/генерация, запись, проверка разделены; комментарии на русском | Обзор кода, `go vet ./...` |
| Тесты каждого модуля | Позитивные и отрицательные случаи | `go test ./...` |
| Форматирование | `gofmt` не предлагает изменений | `gofmt -l .` |
| Good practices | KISS, DRY, разделение ответственности, ошибки обрабатываются; без внешних Go-пакетов | Обзор кода и проверка [официальных правил](https://01.tomorrow-school.ai/api/content/root/01-edu_ai-module/content/good-practices/README.md) |
| Бонус: `--tamper-check` | Регенерация по тому же тексту/шрифту, чтение существующего файла, сравнение по строкам | Команды 10–11; тесты проверки изменений |
| Бонус: номера отличающихся строк | Нумерация начинается с 1; сообщаются все несовпадения, лишние/недостающие линии и изменение последнего LF | Команды 10–11; тесты `internal/verifier` |
| Бонус: неизменённый файл | Сообщение `file matches a fresh regeneration (unmodified)` | Команда 10 до повреждения |
| Бонус: failing verification | Тесты намеренно нарушают число строк, ширину, padding, LF/размер и чтение | `go test ./internal/verifier -v` |
| Бонус: качество сверх минимума | Встроенные шрифты, ограничение больших входов, безопасная запись, негативные CLI-тесты, сохранение существующего файла при проверке изменений | Команды 12–14, обзор тестов и реализации |
| Понимание автором | Можно объяснить запись, перечитывание, пять проверок, формулы и нормальные пробелы | [Ответы аудитору](AUDITOR_ANSWERS.md); ответы должны быть проверены на произвольном примере |

## Команды для повторной проверки

Команды рассчитаны на Bash/Zsh из корня репозитория. Строки с `$` берутся в **одинарные кавычки**, чтобы оболочка не подставила значения переменных. Временный каталог предотвращает перезапись исходных шрифтов и чужих файлов. Его путь можно сохранить до окончания аудита.

### 1. Сборка и автоматические проверки

```sh
audit_dir=$(mktemp -d "${TMPDIR:-/tmp}/symbol-output-audit.XXXXXX")
go build -o "$audit_dir/symbol-output" .
go test ./...
go test -race ./...
go vet ./...
gofmt -l .
go list -m all
```

`gofmt -l .` должен не вывести ни одного имени. `go list -m all` должен показать только текущий модуль. Отсутствие `go.sum` допустимо: внешних модулей нет.

### 2. Одно значение без флагов

```sh
"$audit_dir/symbol-output" 'hello' > "$audit_dir/stdout.txt"
"$audit_dir/symbol-output" --output="$audit_dir/reference.txt" 'hello' standard
cmp "$audit_dir/stdout.txt" "$audit_dir/reference.txt"
```

`cmp` не должен вывести различий. В `stdout.txt` находится рисунок, без служебного сообщения.

### 3. Неправильные аргументы

Каждый вызов должен завершиться с ошибкой и показать Usage; он не должен создавать файл.

```sh
"$audit_dir/symbol-output" --output "$audit_dir/test00.txt" banana standard
"$audit_dir/symbol-output" --output= banana standard
"$audit_dir/symbol-output" --output="$audit_dir/test00.log" banana standard
"$audit_dir/symbol-output" --output="$audit_dir/test00.txt" banana unknown
"$audit_dir/symbol-output" --verify hello standard
"$audit_dir/symbol-output" --output="$audit_dir/test00.txt" --output="$audit_dir/other.txt" hello standard
```

Точные строки Usage:

```text
Usage: go run . [OPTION] [STRING] [BANNER]
EX: go run . --output=<fileName.txt> something standard
```

### 4. Несколько строк и отдельная ширина каждого блока

```sh
"$audit_dir/symbol-output" --output="$audit_dir/test00.txt" 'First\nTest' shadow --verify
cat -e "$audit_dir/test00.txt"
wc -l < "$audit_dir/test00.txt"
```

Ожидаются 16 строк и `5/5 (Valid)`. Первая строка текста длиннее второй, поэтому глобальное требование одной ширины для всего файла было бы ошибкой.

### 5. Точный эталон `hello`

```sh
"$audit_dir/symbol-output" --output="$audit_dir/test01.txt" hello standard
cat -e "$audit_dir/test01.txt"
wc -l < "$audit_dir/test01.txt"
wc -c < "$audit_dir/test01.txt"
"$audit_dir/symbol-output" --output="$audit_dir/test01.txt" hello standard --verify
```

Ожидаются 8 строк, 256 байт, ширина 31 и `Integrity Score: 5/5 (Valid)`. Без `--verify` отчёта нет.

### 6. Специальные символы из аудита

```sh
"$audit_dir/symbol-output" --output="$audit_dir/test02.txt" '123 -> #$%' standard --verify
cat -e "$audit_dir/test02.txt"
"$audit_dir/symbol-output" --output="$audit_dir/thinkertoy-example.txt" 'Go 123 !? @#$%' thinkertoy --verify
```

### 7. Разные буквы, цифры и пробелы

```sh
"$audit_dir/symbol-output" --output="$audit_dir/Mix-2026.txt" 'AbC zYx 019 Tomorrow School' standard --verify
"$audit_dir/symbol-output" --output="$audit_dir/Shadow-2026.txt" 'AbC zYx 019 Tomorrow School' shadow --verify
```

### 8. Весь печатный ASCII во всех шрифтах

```sh
audit_ascii=$(LC_ALL=C awk 'BEGIN { for (i = 32; i <= 126; i++) printf "%c", i }')
for audit_banner in standard shadow thinkertoy; do
  "$audit_dir/symbol-output" --output="$audit_dir/ascii-$audit_banner.txt" "$audit_ascii" "$audit_banner" --verify
done
```

### 9. Пустые строки и настоящие LF

```sh
"$audit_dir/symbol-output" --output="$audit_dir/empty.txt" '' standard --verify
wc -c < "$audit_dir/empty.txt"
"$audit_dir/symbol-output" --output="$audit_dir/blank-rows.txt" '\n' standard --verify
wc -l < "$audit_dir/blank-rows.txt"
"$audit_dir/symbol-output" --output="$audit_dir/trailing-row.txt" 'A\n' standard --verify
wc -l < "$audit_dir/trailing-row.txt"
"$audit_dir/symbol-output" --output="$audit_dir/middle-row.txt" 'A\n\nB' standard --verify
wc -l < "$audit_dir/middle-row.txt"
"$audit_dir/symbol-output" --output="$audit_dir/real-lf.txt" $'A\nB' standard --verify
```

Ожидаются соответственно 0 байт, 16 строк, 16 строк, 24 строки и 16 строк. Для всех результатов структурные проверки должны пройти.

### 10. Бонус: замена символа в строке 4

```sh
"$audit_dir/symbol-output" --output="$audit_dir/tamper.txt" hello standard
"$audit_dir/symbol-output" --output="$audit_dir/tamper.txt" hello standard --tamper-check
awk 'NR == 4 { $0 = "#" substr($0, 2) } { print }' "$audit_dir/tamper.txt" > "$audit_dir/changed.txt"
cp "$audit_dir/changed.txt" "$audit_dir/tamper.txt"
cp "$audit_dir/tamper.txt" "$audit_dir/before-check.txt"
"$audit_dir/symbol-output" --output="$audit_dir/tamper.txt" hello standard --tamper-check
cmp "$audit_dir/before-check.txt" "$audit_dir/tamper.txt"
```

Первая проверка сообщает `unmodified`. После замены должна сообщиться только строка 4. `cmp` подтверждает, что проверка не уничтожила доказательство изменения. У повреждённого файла сохранились размеры, поэтому структурный `--verify` не обязан обнаруживать такую замену.

### 11. Бонус: добавленная пустая строка

```sh
"$audit_dir/symbol-output" --output="$audit_dir/extra-line.txt" hello standard
printf '\n' >> "$audit_dir/extra-line.txt"
"$audit_dir/symbol-output" --output="$audit_dir/extra-line.txt" hello standard --tamper-check
```

Должна сообщиться строка 9. Не следует вместо этого добавлять ещё один перевод строки в эталон, чтобы подогнать вывод под ошибочную иллюстрацию `cat -e`.

### 12. Ошибки файловой системы и входа

```sh
"$audit_dir/symbol-output" --output="$audit_dir/missing/hello.txt" hello standard --verify
mkdir "$audit_dir/directory.txt"
"$audit_dir/symbol-output" --output="$audit_dir/directory.txt" hello standard --verify
"$audit_dir/symbol-output" --output="$audit_dir/unicode.txt" 'Привет' standard
"$audit_dir/symbol-output" --output="$audit_dir/tab.txt" $'A\tB' standard
```

Ожидается понятная ошибка и ненулевой код завершения, без паники и ложного отчёта `Valid`. Непечатные ASCII, кириллица и табуляция не входят в заданные 95 символов шрифтов. Отсутствующий каталог автоматически не создаётся.

### 13. Запуск бинарника вне каталога исходников

```sh
(cd "$audit_dir" && ./symbol-output --output=embedded.txt hello standard --verify)
```

Шрифты включены в бинарник через `embed.FS`, поэтому файл должен успешно создаться независимо от текущего каталога.

### 14. Отказоустойчивость и границы

```sh
go test ./internal/generator -v
go test ./internal/writer -v
go test ./internal/verifier -v
go test ./... -count=10
```

Обзор тестов дополняет запуск: нужны повреждённые таблицы шрифтов, ограничение ввода/результата, все специальные символы, неверные флаги, невозможность записи и негативные проверки файлов. Стресс-проверки должны подтверждать отсутствие паник и предсказуемые ошибки, а не обещать устойчивость при любом объёме памяти или сбое оборудования.

## Вопросы, которые нельзя доказать одним запуском

Автоматические проверки подтверждают поведение программы, но не понимание человека. Для живого аудита нужно уметь без чтения кода объяснить, почему проверяется именно файл с диска, самостоятельно вычислить `8 × число рядов` и `Σ(длина линии + 1)`, различать структурную целостность и точное совпадение содержимого, а также показать, какие пробелы предусмотрены шрифтом. Разбор находится в [ответах аудитору](AUDITOR_ANSWERS.md), а результаты выполненных проверок — в [TEST_REPORT.md](TEST_REPORT.md).
