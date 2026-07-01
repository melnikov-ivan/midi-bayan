// player.js — запись MIDI: накопление событий из уведомлений стандартной BLE MIDI характеристики
// и сохранение их в Standard MIDI File (SMF).

let midiRecording = false;
let midiRecEvents = [];
let midiRecStartTime = 0;

function updateRecordButton() {
    const btn = document.getElementById('recordBtn');
    if (btn) btn.textContent = midiRecording ? '⏹ Стоп записи' : '⏺ Запись';
}

// onMidiNotification разбирает пакет BLE MIDI: [header, timestamp, статус, данные...].
// Прошивка всегда шлёт ровно одно MIDI-сообщение на пакет (см. out.go: sendMidiBLE).
function onMidiNotification(bytes) {
    if (!midiRecording || bytes.length < 3) return;
    const message = Array.from(bytes.slice(2));
    midiRecEvents.push({ message, timeMs: performance.now() - midiRecStartTime });
}

async function toggleRecord() {
    try {
        const msg = buildRecordMessage();
        await BLE.writeValue(msg);
        midiRecording = !midiRecording;
        if (midiRecording) {
            midiRecEvents = [];
            midiRecStartTime = performance.now();
            console.log('Отправлено CMD_RECORD (старт записи)');
        } else {
            console.log('Отправлено CMD_RECORD (стоп записи), событий:', midiRecEvents.length);
            saveMidiRecording();
        }
        updateRecordButton();
    } catch (error) {
        console.error('Ошибка отправки record:', error);
    }
}

// encodeVarLen кодирует число в MIDI variable-length quantity (формат SMF).
function encodeVarLen(value) {
    const buffer = [value & 0x7f];
    value = Math.floor(value / 128);
    while (value > 0) {
        buffer.unshift((value & 0x7f) | 0x80);
        value = Math.floor(value / 128);
    }
    return buffer;
}

// buildMidiFileBytes собирает Standard MIDI File (формат 0, один трек, 120 BPM) из записанных событий.
function buildMidiFileBytes(events) {
    const ppq = 480;
    const tempoUs = 500000; // 120 BPM
    const trackBytes = [0, 0xff, 0x51, 0x03, (tempoUs >> 16) & 0xff, (tempoUs >> 8) & 0xff, tempoUs & 0xff];

    let lastTimeMs = 0;
    for (const ev of events) {
        const deltaMs = ev.timeMs - lastTimeMs;
        lastTimeMs = ev.timeMs;
        const deltaTicks = Math.max(0, Math.round(deltaMs * 1000 * ppq / tempoUs));
        trackBytes.push(...encodeVarLen(deltaTicks));
        trackBytes.push(...ev.message);
    }
    trackBytes.push(0, 0xff, 0x2f, 0x00); // End of Track

    const header = [0x4d, 0x54, 0x68, 0x64, 0, 0, 0, 6, 0, 0, 0, 1, (ppq >> 8) & 0xff, ppq & 0xff];
    const trackLen = trackBytes.length;
    const trackHeader = [
        0x4d, 0x54, 0x72, 0x6b,
        (trackLen >>> 24) & 0xff, (trackLen >>> 16) & 0xff, (trackLen >>> 8) & 0xff, trackLen & 0xff
    ];
    return new Uint8Array([...header, ...trackHeader, ...trackBytes]);
}

function saveMidiRecording() {
    if (midiRecEvents.length === 0) {
        console.log('Нет записанных событий, файл не сохранён');
        return;
    }
    const bytes = buildMidiFileBytes(midiRecEvents);
    const blob = new Blob([bytes], { type: 'audio/midi' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    const ts = new Date().toISOString().replace(/[:.]/g, '-');
    a.href = url;
    a.download = `bayan-recording-${ts}.mid`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    setTimeout(() => URL.revokeObjectURL(url), 1000);
}
