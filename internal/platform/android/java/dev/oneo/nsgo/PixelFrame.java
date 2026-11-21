package dev.oneo.nsgo;

import android.graphics.Bitmap;
import android.util.Log;

import java.io.DataInputStream;
import java.io.IOException;
import java.nio.ByteBuffer;

/** A raw frame from the pipe, held until its mount item on the UI thread. */
final class PixelFrame {
    private final int width, height;
    private final ByteBuffer rgba;

    private PixelFrame(int width, int height, ByteBuffer rgba) {
        this.width = width;
        this.height = height;
        this.rgba = rgba;
    }

    static PixelFrame read(DataInputStream in, byte[] payload) throws IOException {
        int width = in.readInt(), height = in.readInt(), bytes = in.available();
        if (width <= 0 || height <= 0 || bytes % 4 != 0 || (long) width * height != bytes / 4) {
            throw new IOException("wrong dimensions for a pixel frame");
        }
        return new PixelFrame(width, height, ByteBuffer.wrap(payload, payload.length - bytes, bytes));
    }

    long size() { return size(width, height); }

    static long size(int width, int height) {
        return ((long) width << 32) | Integer.toUnsignedLong(height);
    }

    /** ARGB_8888 stores premultiplied RGBA bytes in Android's little-endian memory. */
    Bitmap copyInto(Bitmap bitmap) {
        try {
            if (bitmap == null || bitmap.getWidth() != width || bitmap.getHeight() != height) {
                bitmap = Bitmap.createBitmap(width, height, Bitmap.Config.ARGB_8888);
                bitmap.setDensity(Bitmap.DENSITY_NONE);
            }
            bitmap.copyPixelsFromBuffer(rgba.duplicate());
            return bitmap;
        } catch (RuntimeException | OutOfMemoryError e) {
            Log.w("nsgo", "pixel frame " + width + "x" + height + ": " + e);
            return null;
        }
    }
}
