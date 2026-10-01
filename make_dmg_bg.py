# -*- coding: utf-8 -*-
"""生成 DMG 引导背景图：dmg-bg.png（660x420）+ dmg-bg@2x.png（1320x840，Retina 用）
用法：用项目虚拟环境执行  ./build_env/bin/python make_dmg_bg.py
"""
import os

from PySide6.QtGui import (QGuiApplication, QColor, QPainter, QPen,
                           QFont, QPixmap, QPainterPath)
from PySide6.QtCore import QRectF, QPointF, Qt

app = QGuiApplication([])
W, H = 660, 420
SCALE = 2  # 2x 渲染，Retina 屏文字清晰


def draw(size_w, size_h, scale):
    pm = QPixmap(size_w, size_h)
    pm.fill(QColor('#eef4fb'))
    p = QPainter(pm)
    p.setRenderHint(QPainter.Antialiasing)
    p.setRenderHint(QPainter.TextAntialiasing)
    p.scale(scale, scale)

    p.setPen(QColor('#0f2c52'))
    f = QFont('PingFang SC', 26)
    f.setBold(True)
    p.setFont(f)
    p.drawText(QRectF(0, 48, W, 46), Qt.AlignCenter, '全球鹰 GlobalHawk')
    p.setFont(QFont('PingFang SC', 13))
    p.setPen(QColor('#5b7290'))
    p.drawText(QRectF(0, 100, W, 26), Qt.AlignCenter, '网络空间测绘聚合工具 · v1.0.0')

    def card(x, w, label, sub):
        p.setPen(QPen(QColor('#c4d4ea'), 2))
        p.setBrush(QColor(255, 255, 255, 200))
        p.drawRoundedRect(QRectF(x, 150, w, 190), 18, 18)
        p.setPen(QColor('#41556f'))
        f3 = QFont('PingFang SC', 14)
        f3.setBold(True)
        p.setFont(f3)
        p.drawText(QRectF(x, 352, w, 24), Qt.AlignCenter, label)
        p.setFont(QFont('PingFang SC', 11))
        p.setPen(QColor('#8296ad'))
        p.drawText(QRectF(x, 374, w, 20), Qt.AlignCenter, sub)

    card(60, 200, 'GlobalHawk', '应用')
    card(400, 200, 'Applications', '安装到应用程序')

    p.setPen(Qt.NoPen)
    p.setBrush(QColor('#2f7bd9'))
    a = QPainterPath(QPointF(285, 245))
    a.lineTo(360, 245)
    a.lineTo(360, 225)
    a.lineTo(400, 250)
    a.lineTo(360, 275)
    a.lineTo(360, 255)
    a.lineTo(285, 255)
    a.closeSubpath()
    p.drawPath(a)
    p.setFont(QFont('PingFang SC', 13))
    p.setPen(QColor('#2f7bd9'))
    p.drawText(QRectF(265, 290, 160, 24), Qt.AlignCenter, '拖拽安装')

    p.setFont(QFont('PingFang SC', 11))
    p.setPen(QColor('#8296ad'))
    p.drawText(QRectF(0, 396, W, 20), Qt.AlignCenter,
               '拖动 GlobalHawk 到 Applications 文件夹即可完成安装')
    p.end()
    return pm


out = os.path.join(os.path.dirname(os.path.abspath(__file__)), 'build')
hi = draw(W * SCALE, H * SCALE, SCALE)                 # @2x 原生渲染
hi.save(os.path.join(out, 'dmg-bg@2x.png'))
hi.scaled(W, H, Qt.IgnoreAspectRatio, Qt.SmoothTransformation).save(
    os.path.join(out, 'dmg-bg.png'))
print('已生成 dmg-bg.png（660x420）+ dmg-bg@2x.png（1320x840）')
