const CMD_GET_PROGRAM = 0x01;
const CMD_SET_PROGRAM = 0x02;
const CMD_SET_AUDIO = 0x03;

function crc8(data) {
    let crc = 0;
    for (let i = 0; i < data.length; i++) {
        crc ^= data[i];
        for (let b = 0; b < 8; b++) {
            if (crc & 0x80) {
                crc = ((crc << 1) ^ 0x07) & 0xff;
            } else {
                crc = (crc << 1) & 0xff;
            }
        }
    }
    return crc;
}

function buildGetProgramMessage(channel) {
    const payload = new Uint8Array([channel, 0, 0]);
    const payloadLen = payload.length;
    const msg = new Uint8Array(1 + 2 + payloadLen + 1);
    msg[0] = CMD_GET_PROGRAM;
    msg[1] = payloadLen & 0xff;
    msg[2] = (payloadLen >> 8) & 0xff;
    msg.set(payload, 3);
    msg[3 + payloadLen] = crc8(msg.subarray(0, 3 + payloadLen));
    return msg;
}

function buildSetProgramMessage(channel, instrument, volume, octave) {
    const payload = new Uint8Array([channel, instrument, volume, octave]);
    const payloadLen = payload.length;
    const msg = new Uint8Array(1 + 2 + payloadLen + 1);
    msg[0] = CMD_SET_PROGRAM;
    msg[1] = payloadLen & 0xff;
    msg[2] = (payloadLen >> 8) & 0xff;
    msg.set(payload, 3);
    msg[3 + payloadLen] = crc8(msg.subarray(0, 3 + payloadLen));
    return msg;
}

function buildSetAudioMessage(volume, reverb, chorus, delay) {
    const payload = new Uint8Array([volume & 0xff, reverb & 0xff, chorus & 0xff, delay & 0xff]);
    const payloadLen = payload.length;
    const msg = new Uint8Array(1 + 2 + payloadLen + 1);
    msg[0] = CMD_SET_AUDIO;
    msg[1] = payloadLen & 0xff;
    msg[2] = (payloadLen >> 8) & 0xff;
    msg.set(payload, 3);
    msg[3 + payloadLen] = crc8(msg.subarray(0, 3 + payloadLen));
    return msg;
}
