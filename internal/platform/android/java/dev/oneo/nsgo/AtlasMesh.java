package dev.oneo.nsgo;

import android.graphics.BitmapShader;
import android.graphics.Canvas;
import android.graphics.Paint;
import android.graphics.Shader;
import android.os.Build;

import java.nio.ByteBuffer;
import java.util.Arrays;

/** Draws ordered atlas tiles with one shared bitmap shader per drawing. */
final class AtlasMesh {
    static final int SPRITE_BYTES = 9 * Float.BYTES;
    private static final int CHUNK = 512;
    private static final int VERTICES = 16;
    private static final int TRIANGLE_INDICES = 54;

    private AtlasMesh() {}

    static boolean draw(Canvas canvas, CanvasNode.Picture picture, ByteBuffer source, int count, float alpha) {
        if (Build.VERSION.SDK_INT < 29 || !canvas.isHardwareAccelerated()) return false;
        ByteBuffer sprites = source.duplicate();
        final float density = picture.scale;
        int bitmapWidth = picture.bitmap.getWidth(), bitmapHeight = picture.bitmap.getHeight();
        // Crops outside the bitmap or rotated sprites use drawBitmap's
        // clipping and edge coverage in the single-sprite path.
        for (int i = 0; i < count; i++) {
            float x = sprites.getFloat(), y = sprites.getFloat(), w = sprites.getFloat(), h = sprites.getFloat();
            float c = sprites.getFloat(), s = sprites.getFloat();
            int left = Math.round(x * density), top = Math.round(y * density);
            int right = Math.round((x + w) * density), bottom = Math.round((y + h) * density);
            if (s != 0 || c <= 0 || left < 0 || top < 0 || right > bitmapWidth || bottom > bitmapHeight
                    || left >= right || top >= bottom) return false;
            sprites.position(sprites.position() + 3 * Float.BYTES);
        }

        sprites.position(0);
        int capacity = Math.min(CHUNK, count);
        float[] positions = new float[2 * VERTICES * capacity];
        float[] texels = new float[positions.length];
        int[] colors = new int[VERTICES * capacity];
        short[] indices = indices(capacity);
        Paint paint = new Paint(Paint.FILTER_BITMAP_FLAG);
        paint.setShader(new BitmapShader(picture.bitmap, Shader.TileMode.CLAMP, Shader.TileMode.CLAMP));

        for (int first = 0; first < count; first += capacity) {
            int n = Math.min(capacity, count - first);
            for (int i = 0; i < n; i++) {
                float sx = sprites.getFloat(), sy = sprites.getFloat(), w = sprites.getFloat(), h = sprites.getFloat();
                float c = sprites.getFloat();
                sprites.getFloat(); // axis-aligned
                float x = sprites.getFloat(), y = sprites.getFloat();
                int color = Math.round(sprites.getFloat() * alpha * 255) << 24 | 0x00ffffff;
                int l = Math.round(sx * density), t = Math.round(sy * density);
                int r = Math.round((sx + w) * density), b = Math.round((sy + h) * density);
                int start = VERTICES * i;
                // Keep samples within the crop's outer pixel centers at
                // both edges, including when the destination is enlarged.
                for (int row = 0; row < 4; row++) {
                    float v = grid(t, b, row);
                    float py = y + (v - t) / (b - t) * h * c;
                    for (int col = 0; col < 4; col++) {
                        float u = grid(l, r, col);
                        int at = 2 * (start + 4 * row + col);
                        positions[at] = x + (u - l) / (r - l) * w * c;
                        positions[at + 1] = py;
                        texels[at] = Math.max(l + .5f, Math.min(r - .5f, u));
                        texels[at + 1] = Math.max(t + .5f, Math.min(b - .5f, v));
                    }
                }
                Arrays.fill(colors, start, start + VERTICES, color);
            }
            canvas.drawVertices(Canvas.VertexMode.TRIANGLES, n * VERTICES * 2, positions, 0, texels, 0,
                    colors, 0, indices, 0, n * TRIANGLE_INDICES, paint);
        }
        return true;
    }

    private static float grid(int lo, int hi, int i) {
        switch (i) {
            case 0: return lo;
            case 1: return lo + .5f;
            case 2: return hi - .5f;
            default: return hi;
        }
    }

    private static short[] indices(int count) {
        short[] out = new short[count * TRIANGLE_INDICES];
        int next = 0;
        for (int i = 0; i < count; i++) {
            for (int y = 0; y < 3; y++) {
                for (int x = 0; x < 3; x++) {
                    int first = i * VERTICES + 4 * y + x;
                    out[next++] = (short) first;
                    out[next++] = (short) (first + 1);
                    out[next++] = (short) (first + 4);
                    out[next++] = (short) (first + 1);
                    out[next++] = (short) (first + 5);
                    out[next++] = (short) (first + 4);
                }
            }
        }
        return out;
    }
}
