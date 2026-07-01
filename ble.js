const SERVICE_UUID = '12345678-1234-5678-1234-567890abcdef';
const CHARACTERISTIC_UUID = 'fedcba09-8765-4321-8765-432110325476';

const MIDI_SERVICE_UUID = '03b80e5a-ede8-4b33-a751-6ce34ec4c700';
const MIDI_CHARACTERISTIC_UUID = '7772e5db-3868-4112-a1a9-f2669d106bf3';

let device = null;
let server = null;
let service = null;
let characteristic = null;
let midiService = null;
let midiCharacteristic = null;
let callbacks = null;

async function connect(cbs) {
    callbacks = cbs || {};
    const onConnected = () => (callbacks.onConnected && callbacks.onConnected());
    const onDisconnected = () => (callbacks.onDisconnected && callbacks.onDisconnected());
    const onValue = (bytes) => (callbacks.onValue && callbacks.onValue(bytes));
    const onMidiValue = (bytes) => (callbacks.onMidiValue && callbacks.onMidiValue(bytes));

    try {
        if (!navigator.bluetooth) {
            throw new Error('Web Bluetooth не поддерживается в этом браузере. Используйте Chrome/Edge на десктопе или Android.');
        }

        device = await navigator.bluetooth.requestDevice({
            filters: [{ services: [MIDI_SERVICE_UUID] }],
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

        midiService = await server.getPrimaryService(MIDI_SERVICE_UUID);
        midiCharacteristic = await midiService.getCharacteristic(MIDI_CHARACTERISTIC_UUID);

        midiCharacteristic.addEventListener('characteristicvaluechanged', (event) => {
            const value = event.target.value;
            const buf = value.buffer || value;
            const bytes = new Uint8Array(buf, value.byteOffset || 0, value.byteLength || buf.byteLength);
            onMidiValue(bytes);
        });
        await midiCharacteristic.startNotifications();

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
    midiService = null;
    midiCharacteristic = null;
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

// writeMidiWithoutResponse отправляет BLE MIDI пакет в стандартную MIDI характеристику.
// message — сырое MIDI-сообщение (1–3 байта); оборачивается в header + timestamp.
async function writeMidiWithoutResponse(message) {
    if (!midiCharacteristic) {
        throw new Error('MIDI характеристика не найдена');
    }
    const raw = message instanceof Uint8Array ? message : new Uint8Array(message);
    const ms = Math.floor(performance.now()) % 8192;
    const packet = new Uint8Array(2 + raw.length);
    packet[0] = 0x80 | (ms >> 7);
    packet[1] = 0x80 | (ms & 0x7f);
    packet.set(raw, 2);

    const hex = Array.from(packet).map(b => b.toString(16).padStart(2, '0')).join(' ');
    const props = midiCharacteristic.properties;

    if (props.writeWithoutResponse) {
        await midiCharacteristic.writeValueWithoutResponse(packet);
        console.log('BLE MIDI →', hex);
        return;
    }
    if (props.write) {
        await midiCharacteristic.writeValue(packet);
        console.log('BLE MIDI (write) →', hex);
        return;
    }
    throw new Error('MIDI характеристика не поддерживает запись');
}

window.BLE = {
    connect,
    disconnect,
    readValue,
    writeValue,
    writeMidiWithoutResponse
};
