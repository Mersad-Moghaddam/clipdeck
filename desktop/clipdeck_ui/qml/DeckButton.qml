import QtQuick
import QtQuick.Controls

Button {
    id: control
    property bool primary: false
    padding: 7
    topPadding: 4
    bottomPadding: 4
    font.family: theme.fontFamily
    font.pixelSize: theme.caption
    opacity: enabled ? 1 : 0.45
    contentItem: Text {
        text: control.text
        font: control.font
        color: control.primary ? theme.foreground : (control.hovered ? theme.accent : theme.muted)
        horizontalAlignment: Text.AlignHCenter
        verticalAlignment: Text.AlignVCenter
    }
    background: Rectangle {
        radius: 12
        color: control.primary ? theme.accent : (control.hovered || control.down ? theme.selected : "transparent")
        border.width: control.activeFocus ? 1 : 0
        border.color: theme.accent
    }
}
