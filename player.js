// player.js — запись MIDI и воспроизведение SMF-файлов через BLE MIDI характеристику.

let midiRecording = false;
let midiRecEvents = [];
let midiRecStartTime = 0;

let midiFilePlaying = false;
let midiPlayAbort = false;

function updateRecordButton() {
    const btn = document.getElementById('recordBtn');
    if (btn) btn.textContent = midiRecording ? '⏹ Стоп записи' : '⏺ Запись';
}

function updateMidiPlayButton() {
    const btn = document.getElementById('midiPlayBtn');
    if (btn) btn.textContent = midiFilePlaying ? '⏹ Стоп' : '▶ Пуск';
}

function updateMidiFileLabel() {
    const label = document.getElementById('midiFileName');
    const input = document.getElementById('midiFileInput');
    if (!label || !input) return;
    const file = input.files && input.files[0];
    label.textContent = file ? file.name : 'Файл не выбран';
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
        if (midiRecording) {
            const events = midiRecEvents;
            midiRecEvents = [];
            midiRecording = false;
            updateRecordButton();
            // Сохранение до await BLE: иначе теряется user gesture для showSaveFilePicker.
            const savePromise = saveMidiRecording(events);
            try {
                await BLE.writeValue(msg);
                console.log('Отправлено CMD_RECORD (стоп записи), событий:', events.length);
            } catch (error) {
                console.error('Ошибка отправки record:', error);
            }
            await savePromise;
        } else {
            await BLE.writeValue(msg);
            midiRecording = true;
            midiRecEvents = [];
            midiRecStartTime = performance.now();
            updateRecordButton();
            console.log('Отправлено CMD_RECORD (старт записи)');
        }
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

// saveMidiRecording сохраняет запись. На Android Chrome/PWA (Chrome 132+) — showSaveFilePicker,
// на десктопе — <a download>, на старых мобильных — openBlobForSave.
async function saveMidiRecording(events) {
    if (!events || events.length === 0) {
        console.log('Нет записанных событий, файл не сохранён');
        return;
    }
    const bytes = buildMidiFileBytes(events);
    const ts = new Date().toISOString().replace(/[:.]/g, '-');
    const filename = `bayan-recording-${ts}.mid`;
    const blob = new Blob([bytes], { type: 'audio/midi' });
    const midiTypes = [{
        description: 'MIDI',
        accept: { 'audio/midi': ['.mid'], 'audio/x-midi': ['.mid', '.midi'] },
    }];

    if (window.showSaveFilePicker) {
        try {
            const handle = await window.showSaveFilePicker({ suggestedName: filename, types: midiTypes });
            const writable = await handle.createWritable();
            await writable.write(blob);
            await writable.close();
            return;
        } catch (error) {
            if (error && error.name === 'AbortError') {
                console.log('Сохранение отменено пользователем');
                return;
            }
            console.log('showSaveFilePicker недоступен, пробуем другой способ:', error);
        }
    }

    downloadBlob(blob, filename);
    if (/Android|iPhone|iPad|iPod/i.test(navigator.userAgent)) {
        openBlobForSave(blob, filename);
    }
}

function downloadBlob(blob, filename) {
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    setTimeout(() => URL.revokeObjectURL(url), 1000);
}

// Запасной путь для старых мобильных браузеров без File System Access API.
function openBlobForSave(blob, filename) {
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    a.target = '_blank';
    a.rel = 'noopener noreferrer';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    setTimeout(() => URL.revokeObjectURL(url), 60000);
}

// --- Воспроизведение SMF ---

function readVarLen(bytes, pos) {
    let val = 0;
    let n = 0;
    while (pos + n < bytes.length && n < 4) {
        const c = bytes[pos + n];
        n++;
        val = (val << 7) | (c & 0x7f);
        if ((c & 0x80) === 0) break;
    }
    return { val, n };
}

// parseSmf разбирает Standard MIDI File и возвращает события с абсолютным временем в мс.
function parseSmf(bytes) {
    if (bytes.length < 14) throw new Error('файл слишком короткий');
    if (String.fromCharCode(bytes[0], bytes[1], bytes[2], bytes[3]) !== 'MThd') {
        throw new Error('не SMF');
    }

    const numTracks = (bytes[10] << 8) | bytes[11];
    let ppq = (bytes[12] << 8) | bytes[13];
    if (ppq === 0) ppq = 96;

    let pos = 14;
    const spans = [];
    for (let t = 0; t < numTracks && pos + 8 <= bytes.length; t++) {
        if (String.fromCharCode(bytes[pos], bytes[pos + 1], bytes[pos + 2], bytes[pos + 3]) !== 'MTrk') {
            break;
        }
        const tlen = (bytes[pos + 4] << 24) | (bytes[pos + 5] << 16) | (bytes[pos + 6] << 8) | bytes[pos + 7];
        pos += 8;
        spans.push({ start: pos, end: Math.min(pos + tlen, bytes.length) });
        pos += tlen;
    }
    if (spans.length === 0) throw new Error('нет треков');

    let tempoUs = 500000;
    const allEvents = [];

    for (const span of spans) {
        let trackPos = span.start;
        let absoluteMs = 0;
        let lastStatus = 0;

        while (trackPos < span.end) {
            const { val: delta, n: dn } = readVarLen(bytes, trackPos);
            trackPos += dn;
            if (trackPos >= span.end) break;

            absoluteMs += delta * tempoUs / ppq / 1000;

            if (bytes[trackPos] & 0x80) {
                lastStatus = bytes[trackPos];
                trackPos++;
            }
            if (trackPos >= span.end || lastStatus === 0) break;

            if (lastStatus === 0xff) {
                const metaType = bytes[trackPos++];
                const { val: metaLen, n: mn } = readVarLen(bytes, trackPos);
                trackPos += mn;
                if (metaType === 0x2f) break;
                if (metaType === 0x51 && metaLen === 3 && trackPos + 3 <= span.end) {
                    tempoUs = (bytes[trackPos] << 16) | (bytes[trackPos + 1] << 8) | bytes[trackPos + 2];
                }
                trackPos += metaLen;
                continue;
            }

            if (lastStatus >= 0xf0) {
                const { val: sysexLen, n: sn } = readVarLen(bytes, trackPos);
                trackPos += sn + sysexLen;
                lastStatus = 0;
                continue;
            }

            const msgType = lastStatus & 0xf0;
            let dataLen = 0;
            if (msgType === 0xc0 || msgType === 0xd0) dataLen = 1;
            else if (msgType >= 0x80 && msgType <= 0xe0) dataLen = 2;

            if (trackPos + dataLen > span.end) break;
            const message = new Uint8Array(1 + dataLen);
            message[0] = lastStatus;
            for (let i = 0; i < dataLen; i++) message[1 + i] = bytes[trackPos + i];
            trackPos += dataLen;

            if (msgType >= 0x80 && msgType <= 0xe0) {
                allEvents.push({ timeMs: absoluteMs, message });
            }
        }
    }

    allEvents.sort((a, b) => a.timeMs - b.timeMs);
    return { events: allEvents };
}

function sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}

async function sendAllNotesOffBle() {
    for (let ch = 0; ch < 16; ch++) {
        try {
            await BLE.writeMidiWithoutResponse(new Uint8Array([0xb0 | ch, 123, 0]));
        } catch (error) {
            console.error('All Notes Off failed, ch=', ch, error);
        }
    }
}

async function runMidiPlayback(events) {
    const start = performance.now();
    for (const ev of events) {
        if (midiPlayAbort) return;
        const elapsed = performance.now() - start;
        const delay = ev.timeMs - elapsed;
        if (delay > 0) {
            await sleep(delay);
            if (midiPlayAbort) return;
        }
        await BLE.writeMidiWithoutResponse(ev.message);
    }
}

function stopMidiPlayback() {
    midiPlayAbort = true;
    midiFilePlaying = false;
    updateMidiPlayButton();
    sendAllNotesOffBle();
}

async function toggleMidiPlayback() {
    if (midiFilePlaying) {
        stopMidiPlayback();
        return;
    }

    const input = document.getElementById('midiFileInput');
    const file = input && input.files && input.files[0];
    if (!file) {
        alert('Выберите MIDI-файл');
        return;
    }

    let parsed;
    try {
        const bytes = new Uint8Array(await file.arrayBuffer());
        parsed = parseSmf(bytes);
    } catch (error) {
        alert('Не удалось разобрать MIDI: ' + error.message);
        return;
    }

    if (!parsed.events.length) {
        alert('В файле нет MIDI-событий');
        return;
    }

    midiPlayAbort = false;
    midiFilePlaying = true;
    updateMidiPlayButton();
    console.log('Воспроизведение MIDI, событий:', parsed.events.length);

    try {
        await runMidiPlayback(parsed.events);
    } catch (error) {
        console.error('Ошибка воспроизведения:', error);
    } finally {
        if (!midiPlayAbort) {
            midiFilePlaying = false;
            updateMidiPlayButton();
        }
    }
}
