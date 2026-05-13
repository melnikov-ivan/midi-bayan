const SERVICE_UUID = '12345678-1234-5678-1234-567890abcdef';
const CHARACTERISTIC_UUID = 'fedcba09-8765-4321-8765-432110325476';

let device = null;
let server = null;
let service = null;
let characteristic = null;
let callbacks = null;

async function connect(cbs) {
    callbacks = cbs || {};
    const onConnected = () => (callbacks.onConnected && callbacks.onConnected());
    const onDisconnected = () => (callbacks.onDisconnected && callbacks.onDisconnected());
    const onValue = (bytes) => (callbacks.onValue && callbacks.onValue(bytes));

    try {
        if (!navigator.bluetooth) {
            throw new Error('Web Bluetooth не поддерживается в этом браузере. Используйте Chrome/Edge на десктопе или Android.');
        }

        device = await navigator.bluetooth.requestDevice({
            filters: [{ services: ['03b80e5a-ede8-4b33-a751-6ce34ec4c700'] }],
            optionalServices: [SERVICE_UUID]
        });

        device.addEventListener('gattserverdisconnected', () => {
            handleDisconnected();
            onDisconnected();
        });

        server = await device.gatt.connect();
        service = await server.getPrimaryService(SERVICE_UUID);
        characteristic = await service.getCharacteristic(CHARACTERISTIC_UUID);

        characteristic.addEventListener('characteristicvaluechanged', (event) => {
            const value = event.target.value;
            const buf = value.buffer || value;
            const bytes = new Uint8Array(buf, value.byteOffset || 0, value.byteLength || buf.byteLength);
            onValue(bytes);
        });
        await characteristic.startNotifications();

        onConnected();
        return true;
    } catch (error) {
        console.error('Ошибка подключения:', error);
        clearState();
        return false;
    }
}

function handleDisconnected() {
    console.log('Устройство отключено');
    clearState();
}

function clearState() {
    device = null;
    server = null;
    service = null;
    characteristic = null;
}

function disconnect() {
    if (device && device.gatt.connected) {
        device.gatt.disconnect();
    }
    handleDisconnected();
    if (callbacks && callbacks.onDisconnected) {
        callbacks.onDisconnected();
    }
}

async function readValue() {
    if (!characteristic) {
        throw new Error('Характеристика не найдена');
    }
    const value = await characteristic.readValue();
    const buf = value.buffer || value;
    return new Uint8Array(buf, value.byteOffset || 0, value.byteLength || buf.byteLength);
}

async function writeValue(data) {
    if (!characteristic) {
        throw new Error('Характеристика не найдена');
    }
    const buffer = data instanceof Uint8Array ? data.buffer : data;
    await characteristic.writeValue(buffer);
}

window.BLE = {
    connect,
    disconnect,
    readValue,
    writeValue
};
