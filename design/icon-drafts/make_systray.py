#!/usr/bin/env python3
"""生成状态栏图标预览：黑template / 黑+绿点激活态，叠深浅菜单栏底对比。"""
import os
from PIL import Image
import numpy as np
from make_icons import extract_silhouette

HERE = os.path.dirname(os.path.abspath(__file__))


def make_tray_icon(sil, color, dot=False, dot_color=(34, 197, 94), size=88,
                   person_ratio=0.95):
    """
    sil: bool 2D 剪影（原始裁剪坐标）
    color: 剪影填充色 (r,g,b)
    dot: 是否在帽顶加圆点
    person_ratio: 人物高度占画布比例
    返回 RGBA Image
    """
    ch, cw = sil.shape
    target_h = int(size * person_ratio)
    scale = target_h / ch
    nw = int(round(cw * scale))
    nh = target_h
    from PIL import Image as IM
    small = IM.fromarray((sil * 255).astype(np.uint8)).resize((nw, nh), IM.LANCZOS)
    m = np.array(small) > 127

    rgba = np.zeros((size, size, 4), np.uint8)
    # 居中放置
    top = (size - nh) // 2
    left = (size - nw) // 2
    # 填充剪影
    region = rgba[top:top + nh, left:left + nw]
    region[m] = (*color, 255)

    if dot:
        # 帽身下沿（再下移 0.75 个绿点直径）：原始坐标 y≈113，x 中心 156
        hy = int(113 / 352 * nh) + top
        hx = int(156 / 312 * nw) + left
        # 圆点半径：缩小到帽宽的 7%
        r = max(2, int((102 / 312 * nw) * 0.07))
        # 画填充圆 + 浅描边（深浅底都可见）
        for dy in range(-r - 2, r + 3):
            for dx in range(-r - 2, r + 3):
                d = (dx * dx + dy * dy) ** 0.5
                yy, xx = hy + dy, hx + dx
                if 0 <= yy < size and 0 <= xx < size:
                    if d <= r:
                        rgba[yy, xx] = (*dot_color, 255)
                    elif d <= r + 1.5:
                        # 细描边（白色半透明，确保深底可见）
                        rgba[yy, xx] = (255, 255, 255, 200)
    return Image.fromarray(rgba, "RGBA")


def on_menu_bar(icon, light=True):
    """把图标叠在模拟菜单栏底色上。"""
    bg = (236, 236, 236) if light else (38, 38, 38)
    canvas = Image.new("RGBA", (icon.width + 20, icon.height + 16), (*bg, 255))
    canvas.alpha_composite(icon, (10, 8))
    return canvas


if __name__ == "__main__":
    sil = extract_silhouette()
    black = (0, 0, 0)
    # 待机：黑色 template
    idle = make_tray_icon(sil, black, dot=False)
    # 激活：黑色 + 绿点
    active = make_tray_icon(sil, black, dot=True)

    # 深浅底对比拼图
    s = idle.width
    W = s * 2 + 50
    H = s + 36
    combo = Image.new("RGBA", (W, H * 2 + 40), (255, 255, 255, 0))
    # 上排：浅底
    light_idle = on_menu_bar(idle, light=True)
    light_active = on_menu_bar(active, light=True)
    combo.alpha_composite(light_idle, (10, 8))
    combo.alpha_composite(light_active, (s + 30, 8))
    # 下排：深底
    dark_idle = on_menu_bar(idle, light=False)
    dark_active = on_menu_bar(active, light=False)
    combo.alpha_composite(dark_idle, (10, H + 20))
    combo.alpha_composite(dark_active, (s + 30, H + 20))

    combo.save(os.path.join(HERE, "preview-systray-dot.png"))
    idle.save(os.path.join(HERE, "preview-systray-idle.png"))
    active.save(os.path.join(HERE, "preview-systray-active.png"))
    print("ok, dot radius ~", max(2, int((102 / 312 * int(s * 0.66 / 352 * 312)) * 0.12)))
