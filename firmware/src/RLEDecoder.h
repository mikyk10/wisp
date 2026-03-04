#ifndef RLE_DECODER_H
#define RLE_DECODER_H

#include <Arduino.h>
#include <stdint.h>

/**
 * RLE Decoder for streaming decompression of RLE-compressed binary data.
 * Format: [byte_value, count] pairs (each 1 byte, max run 255)
 */
class RLEDecoder {
public:
    RLEDecoder(const uint8_t* compressedData, size_t compressedSize)
        : data(compressedData), size(compressedSize), pos(0), runPos(0), runByte(0), runCount(0) {}

    /**
     * Decompress and call callback for each byte
     * @param callback Function to call with each decompressed byte
     */
    template <typename Callback>
    void decode(Callback callback) {
        while (pos < size) {
            uint8_t byte = pgm_read_byte(&data[pos++]);
            uint8_t count = pgm_read_byte(&data[pos++]);

            for (uint8_t i = 0; i < count; i++) {
                callback(byte);
            }
        }
    }

    /**
     * Get next decompressed byte
     * @return Decompressed byte, or -1 if end of data
     */
    int nextByte() {
        // If we have remaining bytes in current run, return next one
        if (runPos < runCount) {
            uint8_t byte = runByte;
            runPos++;
            return byte;
        }

        // No more data
        if (pos >= size) {
            return -1;
        }

        // Read next [byte, count] pair
        runByte = pgm_read_byte(&data[pos++]);
        runCount = pgm_read_byte(&data[pos++]);
        runPos = 0;

        if (runCount > 0) {
            runPos++;
            return runByte;
        }

        return nextByte();  // Skip empty runs
    }

    /**
     * Reset decoder to beginning
     */
    void reset() {
        pos = 0;
        runPos = 0;
        runByte = 0;
        runCount = 0;
    }

private:
    const uint8_t* data;
    size_t size;
    size_t pos;        // Position in compressed data
    uint8_t runPos;    // Position in current run
    uint8_t runByte;   // Current byte value
    uint8_t runCount;  // Count of current run
};

#endif // RLE_DECODER_H
