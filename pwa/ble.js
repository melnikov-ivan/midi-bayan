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
let midiWriteWithoutResponse = false;
let midiWrite = false;
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

        const midiProps = midiCharacteristic.properties;
        midiWriteWithoutResponse = midiProps.writeWithoutResponse;
        midiWrite = midiProps.write;

        resetMidiQueue();

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
    midiWriteWithoutResponse = false;
    midiWrite = false;
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

let midiWriteChain = Promise.resolve();
const MIDI_WRITE_GAP_MS = 2;
const MAX_MSGS_PER_PACKET = 4;

function resetMidiQueue() {
    midiWriteChain = Promise.resolve();
}

function sleepMs(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}

function buildBleMidiPacket(messages) {
    const ms = Math.floor(performance.now()) % 8192;
    let size = 1;
    for (const msg of messages) size += 1 + msg.length;
    const packet = new Uint8Array(size);
    packet[0] = 0x80 | (ms >> 7);
    let off = 1;
    for (const msg of messages) {
        packet[off++] = 0x80 | (ms & 0x7f);
        packet.set(msg, off);
        off += msg.length;
    }
    return packet;
}

function buildSingleBleMidiPacket(message) {
    const raw = message instanceof Uint8Array ? message : new Uint8Array(message);
    const ms = Math.floor(performance.now()) % 8192;
    const packet = new Uint8Array(2 + raw.length);
    packet[0] = 0x80 | (ms >> 7);
    packet[1] = 0x80 | (ms & 0x7f);
    packet.set(raw, 2);
    return packet;
}

async function sendBleMidiPacket(packet) {
    if (midiWriteWithoutResponse) {
        try {
            await midiCharacteristic.writeValueWithoutResponse(packet);
            return;
        } catch (error) {
            console.warn('writeWithoutResponse failed:', error);
        }
    }
    if (midiWrite) {
        await midiCharacteristic.writeValue(packet);
        return;
    }
    await midiCharacteristic.writeValueWithoutResponse(packet);
}

function enqueueBleMidiWrite(task) {
    const done = midiWriteChain.then(async () => {
        await task();
        await sleepMs(MIDI_WRITE_GAP_MS);
    });
    midiWriteChain = done.catch((err) => console.error('BLE MIDI write:', err));
    return done;
}

function drainMidiQueue() {
    return midiWriteChain;
}

async function writeMidiWithoutResponse(message) {
    if (!midiCharacteristic) {
        throw new Error('MIDI характеристика не найдена');
    }
    await enqueueBleMidiWrite(async () => {
        await sendBleMidiPacket(buildSingleBleMidiPacket(message));
    });
}

async function writeMidiBatch(messages) {
    if (!midiCharacteristic || !messages.length) return;
    for (let i = 0; i < messages.length; i += MAX_MSGS_PER_PACKET) {
        const chunk = messages.slice(i, i + MAX_MSGS_PER_PACKET);
        await enqueueBleMidiWrite(async () => {
            const packet = chunk.length === 1
                ? buildSingleBleMidiPacket(chunk[0])
                : buildBleMidiPacket(chunk);
            await sendBleMidiPacket(packet);
        });
    }
}

window.BLE = {
    connect,
    disconnect,
    readValue,
    writeValue,
    writeMidiWithoutResponse,
    writeMidiBatch,
    drainMidiQueue,
    resetMidiQueue
};
