#!/bin/sh
# Проверки запускаются в отдельном каталоге и не меняют файлы проекта.
set -eu

script_directory=$(CDPATH= cd "$(dirname "$0")" && pwd)
repository_directory=$(CDPATH= cd "$script_directory/.." && pwd)
cd "$repository_directory"

audit_directory=$(mktemp -d "${TMPDIR:-/tmp}/symbol-output-audit.XXXXXX")
trap 'rm -rf "$audit_directory"' 0
trap 'exit 1' 1 2 3 15
audit_binary="$audit_directory/symbol-output"

fail() {
    printf 'Ошибка аудита: %s\n' "$*" >&2
    exit 1
}

# Проверяем код завершения именно готовой программы: go run меняет его.
run_expect() {
    audit_expected_status=$1
    audit_case_name=$2
    shift 2
    if "$@" > "$audit_case_name.stdout" 2> "$audit_case_name.stderr"; then
        audit_actual_status=0
    else
        audit_actual_status=$?
    fi
    if [ "$audit_actual_status" -ne "$audit_expected_status" ]; then
        cat "$audit_case_name.stderr" >&2
        fail "$audit_case_name: код $audit_actual_status вместо $audit_expected_status"
    fi
}

assert_empty() {
    [ ! -s "$1" ] || fail "ожидался пустой файл $1"
}

go build -o "$audit_binary" .
[ -f "$repository_directory/test01.txt" ] || fail 'не найден эталон test01.txt'

# В рабочем каталоге нет таблиц шрифтов: программа должна использовать embed.
cd "$audit_directory"
run_expect 0 hello "$audit_binary" --output=hello.txt hello standard
assert_empty hello.stdout
assert_empty hello.stderr
cmp hello.txt "$repository_directory/test01.txt" || fail 'hello отличается от эталона'
[ "$(wc -c < hello.txt)" -eq 256 ] || fail 'hello должен занимать 256 байтов'
[ "$(wc -l < hello.txt)" -eq 8 ] || fail 'hello должен содержать 8 строк'
awk 'length($0) != 31 { exit 1 }' hello.txt || fail 'ширина hello должна быть 31'

run_expect 0 verify "$audit_binary" --output=hello.txt hello standard --verify
cat > verify.expected <<'EOF'
File created: hello.txt

--- Verification Report ---
Integrity Score: 5/5 (Valid)

Checks:
  [PASS] File is readable
  [PASS] Line count is 8 (8 per row x 1 row)
  [PASS] Width consistent within each row (31)
  [PASS] No padding beyond row width
  [PASS] File size matches (256 bytes)
EOF
cmp verify.stdout verify.expected || fail 'отчёт --verify отличается от задания'
grep -F -x 'Integrity Score: 5/5 (Valid)' verify.stdout > /dev/null
assert_empty verify.stderr

run_expect 2 invalid "$audit_binary" --output result.txt hello
cat > usage.expected <<'EOF'
Usage: go run . [OPTION] [STRING] [BANNER]
EX: go run . --output=<fileName.txt> something standard
EOF
cmp invalid.stdout usage.expected || fail 'неверный формат --output должен печатать Usage'
assert_empty invalid.stderr
[ ! -e result.txt ] || fail 'неверный флаг создал файл'

for audit_banner in standard shadow thinkertoy; do
    run_expect 0 "stdout-$audit_banner" "$audit_binary" 'Hello 01! ~@#$%^&*()_+-=[]{};:,.?/\' "$audit_banner"
    run_expect 0 "file-$audit_banner" "$audit_binary" "--output=art-$audit_banner.txt" 'Hello 01! ~@#$%^&*()_+-=[]{};:,.?/\' "$audit_banner"
    cmp "art-$audit_banner.txt" "stdout-$audit_banner.stdout" || fail "$audit_banner: файл отличается от stdout"
    assert_empty "stdout-$audit_banner.stderr"
    assert_empty "file-$audit_banner.stdout"
    assert_empty "file-$audit_banner.stderr"
    run_expect 0 "verify-$audit_banner" "$audit_binary" "--output=art-$audit_banner.txt" 'Hello 01! ~@#$%^&*()_+-=[]{};:,.?/\' "$audit_banner" --verify
    grep -F -x 'Integrity Score: 5/5 (Valid)' "verify-$audit_banner.stdout" > /dev/null
    assert_empty "verify-$audit_banner.stderr"
done

# Настоящий перенос и буквальная пара \n должны давать одинаковый рисунок.
run_expect 0 literal_newlines "$audit_binary" 'A\n\nB' standard
run_expect 0 real_newlines "$audit_binary" "$(printf 'A\n\nB')" standard
cmp literal_newlines.stdout real_newlines.stdout || fail 'виды переноса дают разный результат'
[ "$(wc -l < literal_newlines.stdout)" -eq 24 ] || fail 'средняя пустая строка потеряна'
assert_empty literal_newlines.stderr
assert_empty real_newlines.stderr
run_expect 0 blank_rows "$audit_binary" --output=blank.txt '\n' standard --verify
[ "$(wc -l < blank.txt)" -eq 16 ] || fail 'две пустые текстовые строки должны давать 16 строк'
[ "$(wc -c < blank.txt)" -eq 16 ] || fail 'пустые строки должны содержать только переносы'
grep -F -x 'Integrity Score: 5/5 (Valid)' blank_rows.stdout > /dev/null
assert_empty blank_rows.stderr
run_expect 0 empty_input "$audit_binary" --output=empty.txt '' standard --verify
assert_empty empty.txt
grep -F -x 'Integrity Score: 5/5 (Valid)' empty_input.stdout > /dev/null
assert_empty empty_input.stderr

run_expect 0 clean_tamper "$audit_binary" --output=hello.txt hello standard --tamper-check
grep -F -x 'Result: file matches a fresh regeneration (unmodified)' clean_tamper.stdout > /dev/null
cmp hello.txt "$repository_directory/test01.txt" || fail 'проверка изменила исходный файл'
assert_empty clean_tamper.stderr
cp hello.txt hello.clean.txt

# Меняется знак в строке 4; пробелы, ширина и число байтов сохраняются.
awk 'NR == 4 { $0 = "#" substr($0, 2) } { print }' hello.clean.txt > hello.txt
cp hello.txt tampered.expected.txt
run_expect 1 changed_tamper "$audit_binary" --output=hello.txt hello standard --tamper-check
grep -F -x '  Line 4 differs' changed_tamper.stdout > /dev/null
grep -F -x 'Result: 1 line differs from a fresh regeneration' changed_tamper.stdout > /dev/null
cmp hello.txt tampered.expected.txt || fail '--tamper-check скрыл изменение перезаписью'
assert_empty changed_tamper.stderr

# Геометрия ещё правильная, хотя содержимое уже изменено: проверки независимы.
run_expect 1 combined_tamper "$audit_binary" --output=hello.txt hello standard --verify --tamper-check
grep -F -x 'Integrity Score: 5/5 (Valid)' combined_tamper.stdout > /dev/null
grep -F -x '  Line 4 differs' combined_tamper.stdout > /dev/null
cmp hello.txt tampered.expected.txt || fail 'совместная проверка изменила улику'
assert_empty combined_tamper.stderr

cp hello.clean.txt hello.txt
printf '\n' >> hello.txt
cp hello.txt extra_line.expected.txt
run_expect 1 extra_line "$audit_binary" --output=hello.txt hello standard --tamper-check
grep -F -x '  Line 9 differs' extra_line.stdout > /dev/null
cmp hello.txt extra_line.expected.txt || fail 'проверка лишней строки перезаписала файл'
assert_empty extra_line.stderr

run_expect 1 missing_file "$audit_binary" --output=missing.txt hello standard --verify --tamper-check
[ ! -e missing.txt ] || fail 'проверка отсутствующего файла создала его'
assert_empty missing_file.stdout
[ -s missing_file.stderr ] || fail 'отсутствующий файл должен сопровождаться ошибкой'

printf 'Аудит пройден: шрифты, формат, отчёт, переносы и обнаружение изменений.\n'
