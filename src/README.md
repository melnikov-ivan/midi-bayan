код написан на tinygo.

1. создаем модуль в этом каталоге
`go mod init bayan`

2. пишем код 
здесь же (пакет `main`, рядом `go.mod`)

3. загружаем библиотеку
`go get tinygo.org/x/bluetooth`

4. прошиваем
переводим плату в boot mode двойным нажатием
`make flash` или `tinygo flash -target=xiao-ble -tags=ble .`

5. сборка
`tinygo build -target=xiao-ble -o ../firmware.uf2 .` посмотреть размер бинарника

6. отладка
читаем логи в терминале 
`make monitor` или `tinygo monitor`
