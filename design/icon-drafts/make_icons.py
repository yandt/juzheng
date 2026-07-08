#!/usr/bin/env python3
"""
生成最终 App 图标与状态栏图标资源。
- 共用：从 reference.jpg 抠出干净的人物剪影（flood-fill 补内部空洞 + 高斯平滑）
- appicon.png: 圆角方底(squircle) + 朱红渐变 + 赭金人物，1024
- systray-template.png: 纯黑透明剪影（macOS template），居中，留 padding，44/88
- 预览：preview-appicon.png
"""
import os
from PIL import Image, ImageFilter
import numpy as np
from collections import deque

HERE = os.path.dirname(os.path.abspath(__file__))
PROJECT = os.path.dirname(os.path.dirname(HERE))
SRC = os.path.join(HERE, "reference.jpg")

YELLOW = (230, 188, 79)  # #E6BC4F 赭金


def extract_silhouette():
    g = np.array(Image.open(SRC).convert("L"))
    fg = g < 120
    H, W = fg.shape
    visited = np.zeros_like(fg, dtype=bool)
    q = deque()

    def seed(i, j):
        if not fg[i, j] and not visited[i, j]:
            visited[i, j] = True
            q.append((i, j))

    for i in range(H):
        seed(i, 0)
        seed(i, W - 1)
    for j in range(W):
        seed(0, j)
        seed(H - 1, j)
    while q:
        i, j = q.popleft()
        for di, dj in ((-1, 0), (1, 0), (0, -1), (0, 1)):
            ni, nj = i + di, j + dj
            if 0 <= ni < H and 0 <= nj < W and not fg[ni, nj] and not visited[ni, nj]:
                visited[ni, nj] = True
                q.append((ni, nj))
    person = ~visited
    sm = Image.fromarray((person * 255).astype(np.uint8)).filter(
        ImageFilter.GaussianBlur(2.5)
    )
    person_sm = np.array(sm) > 127
    # 裁剪到 bbox + padding
    ys, xs = np.where(person_sm)
    pad = 8
    x0, x1 = max(xs.min() - pad, 0), min(xs.max() + pad, W)
    y0, y1 = max(ys.min() - pad, 0), min(ys.max() + pad, H)
    return person_sm[y0:y1, x0:x1]


def squircle_mask(size, n=5):
    """iOS 风格超椭圆圆角方底 mask。"""
    yy, xx = np.mgrid[0:size, 0:size]
    cx = cy = size / 2
    a = b = size / 2
    return (np.abs((xx - cx) / a) ** n + np.abs((yy - cy) / b) ** n) <= 1


def make_appicon(sil, out_path, size=1024, person_ratio=0.64, yellow=YELLOW,
                 c1=(200, 44, 44), c2=(140, 24, 24)):
    ch, cw = sil.shape
    # 人物高度约占画面 person_ratio，垂直略上移
    target_h = int(size * person_ratio)
    scale = target_h / ch
    nw = int(round(cw * scale))
    nh = target_h
    small = Image.fromarray((sil * 255).astype(np.uint8)).resize(
        (nw, nh), Image.LANCZOS
    )
    m = np.array(small) > 127

    bg_mask = squircle_mask(size, n=5)
    yy = np.arange(size).reshape(-1, 1)
    # 朱红→深红 垂直渐变
    c1 = np.array([200, 44, 44])  # #C82C2C
    c2 = np.array([140, 24, 24])  # #8C1818
    t = (yy / size) * np.ones((1, size))
    grad = np.zeros((size, size, 3), np.uint8)
    for k in range(3):
        grad[..., k] = (c1[k] * (1 - t * 0.7) + c2[k] * (t * 0.7)).astype(np.uint8)

    rgba = np.dstack([grad, (bg_mask * 255).astype(np.uint8)])
    out = Image.fromarray(rgba, "RGBA")
    # 人物居中略上移
    top = int((size - nh) / 2) - int(size * 0.03)
    left = (size - nw) // 2
    person_rgba = np.zeros((nh, nw, 4), np.uint8)
    person_rgba[m] = (*yellow, 255)
    out.alpha_composite(Image.fromarray(person_rgba, "RGBA"), (left, top))
    out.save(out_path)
    print("wrote", out_path, out.size)
    return out


def make_systray_template(sil, out_path, size=88):
    """纯黑 + 透明 template 剪影。状态栏要求：实心、单色、留边距。"""
    ch, cw = sil.shape
    # 留 ~18% padding，让剪影不顶满
    target_h = int(size * 0.64)
    scale = target_h / ch
    nw = int(round(cw * scale))
    nh = target_h
    small = Image.fromarray((sil * 255).astype(np.uint8)).resize(
        (nw, nh), Image.LANCZOS
    )
    m = np.array(small) > 127
    # 先在小尺寸上填黑，再贴到 size×size 透明画布
    person_rgba = np.zeros((nh, nw, 4), np.uint8)
    person_rgba[m] = (0, 0, 0, 255)
    canvas = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    canvas.alpha_composite(Image.fromarray(person_rgba, "RGBA"),
                           ((size - nw) // 2, (size - nh) // 2))
    canvas.save(out_path)
    print("wrote", out_path, canvas.size)
    return canvas


if __name__ == "__main__":
    sil = extract_silhouette()
    # 预览
    make_appicon(sil, os.path.join(HERE, "preview-appicon.png"), 1024)
    # 状态栏 template 预览（叠在浅灰底上方便看）
    tpl = make_systray_template(sil, os.path.join(HERE, "preview-systray.png"), 88)
