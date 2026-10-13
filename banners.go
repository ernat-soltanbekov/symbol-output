package main

import "embed"

// Шрифты попадают в исполняемый файл: запуск не зависит от текущего каталога.
//
//go:embed standard.txt shadow.txt thinkertoy.txt
var bannerFiles embed.FS
