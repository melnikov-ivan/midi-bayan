
TARGET ?= xiao-ble

flash:
	tinygo flash -target=$(TARGET) .

build:
	tinygo build -target=xiao-ble -size short -o /tmp/firmware.uf2; \
	cp -X /tmp/firmware.uf2 /Volumes/XIAO-SENSE/

monitor:
	tinygo monitor

.PHONY: flash monitor
