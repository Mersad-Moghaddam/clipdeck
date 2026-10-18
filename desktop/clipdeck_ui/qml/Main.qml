import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import QtQuick.Window

Window {
    id: window
    title: "ClipDeck"
    width: 1060; height: 700
    minimumWidth: 720; minimumHeight: 440
    visible: deck.visible
    color: theme.background
    flags: Qt.Window | Qt.FramelessWindowHint
    onVisibleChanged: if (visible) requestActivate()

    Shortcut { sequence: "Escape"; onActivated: deck.dismiss() }
    Shortcut { sequence: "Ctrl+C"; onActivated: if (history.currentItem) deck.copy_item(history.currentItem.clip.id) }
    Shortcut { sequence: "Return"; onActivated: if (history.currentItem) deck.paste_item(history.currentItem.clip.id) }
    Shortcut { sequence: "Ctrl+P"; onActivated: if (history.currentItem) deck.pin_item(history.currentItem.clip.id, !history.currentItem.clip.pinned) }
    Shortcut { sequence: "Ctrl+L"; onActivated: search.forceActiveFocus() }

    Rectangle {
        anchors.fill: parent
        color: "transparent"; border.color: theme.accent; border.width: 1; radius: theme.radius
        ColumnLayout {
            anchors.fill: parent; anchors.margins: 12; spacing: 12
            RowLayout {
                Layout.fillWidth: true; spacing: 12
                Rectangle {
                    width: 30; height: 30; radius: 8; color: theme.accent
                    Text { anchors.centerIn: parent; text: "☰"; color: theme.background; font.pixelSize: 20 }
                }
                ColumnLayout {
                    spacing: 2
                    Text { text: "ClipDeck"; color: theme.foreground; font.family: theme.fontFamily; font.pixelSize: theme.title; font.bold: true }
                    Text { text: deck.items.length + " clips · instant search"; color: theme.muted; font.family: theme.fontFamily; font.pixelSize: theme.caption }
                }
                TextField {
                    id: search
                    Layout.fillWidth: true
                    placeholderText: "Type to search…  Backspace edits, Ctrl+U clears"
                    color: theme.foreground; placeholderTextColor: theme.muted
                    font.family: theme.fontFamily; font.pixelSize: theme.body
                    background: Rectangle { color: theme.surface; radius: theme.radius }
                    onTextChanged: searchDelay.restart()
                    Keys.onDownPressed: { history.forceActiveFocus(); history.currentIndex = Math.max(0, history.currentIndex) }
                }
                Text { text: deck.items.length + " shown"; color: theme.muted; font.family: theme.fontFamily; font.pixelSize: theme.caption }
                Button { text: "×"; onClicked: deck.dismiss() }
            }
            RowLayout {
                id: categories
                property string selected: "All"
                spacing: 8
                Repeater {
                    model: ["All", "Pinned", "Colors", "Code", "Links", "Images", "Text"]
                    delegate: Button {
                        required property string modelData
                        text: modelData
                        font.family: theme.fontFamily; font.pixelSize: theme.caption
                        contentItem: Text { text: parent.text; color: categories.selected === text ? theme.accent : theme.foreground; font: parent.font; horizontalAlignment: Text.AlignHCenter }
                        background: Rectangle { color: categories.selected === parent.text ? theme.selected : "transparent"; radius: 12 }
                        onClicked: { categories.selected = modelData; deck.search(search.text, modelData) }
                    }
                }
                Item { Layout.fillWidth: true }
            }
            Rectangle { Layout.fillWidth: true; height: 1; color: theme.border }
            RowLayout {
                Layout.fillWidth: true; Layout.fillHeight: true; spacing: 16
                ListView {
                    id: history
                    Layout.preferredWidth: window.width * 0.40
                    Layout.fillHeight: true
                    clip: true; model: deck.items; spacing: 3
                    currentIndex: -1
                    onCurrentIndexChanged: if (currentItem) deck.select_item(currentItem.clip)
                    delegate: Rectangle {
                        id: row
                        required property var modelData
                        required property int index
                        property var clip: modelData
                        width: history.width; height: 51; radius: 4
                        color: history.currentIndex === index ? theme.selected : "transparent"
                        Rectangle { width: 3; anchors.left: parent.left; height: parent.height - 12; anchors.verticalCenter: parent.verticalCenter; color: history.currentIndex === row.index ? theme.accent : "transparent" }
                        RowLayout {
                            anchors.fill: parent; anchors.margins: 8; spacing: 10
                            Rectangle { width: 30; height: 30; radius: 6; color: theme.surface
                                Text { anchors.centerIn: parent; text: (row.clip.type || "TXT").slice(0,3).toUpperCase(); color: theme.foreground; font.family: theme.fontFamily; font.pixelSize: theme.caption }
                            }
                            ColumnLayout { Layout.fillWidth: true; spacing: 3
                                Text { Layout.fillWidth: true; text: (row.clip.pinned ? "PINNED " : "") + (row.clip.source_app || row.clip.type || "clipboard"); textFormat: Text.PlainText; elide: Text.ElideRight; color: row.clip.pinned ? theme.accent : theme.muted; font.family: theme.fontFamily; font.pixelSize: theme.caption }
                                Text { Layout.fillWidth: true; text: row.clip.preview || row.clip.content || ""; textFormat: Text.PlainText; elide: Text.ElideRight; maximumLineCount: 1; color: theme.foreground; font.family: theme.fontFamily; font.pixelSize: theme.body }
                            }
                        }
                        MouseArea { anchors.fill: parent; onClicked: { history.currentIndex = row.index; deck.select_item(row.clip) } onDoubleClicked: deck.copy_item(row.clip.id) }
                    }
                    ScrollBar.vertical: ScrollBar {}
                    Keys.onReturnPressed: if (currentItem) deck.paste_item(currentItem.clip.id)
                }
                Rectangle { width: 1; Layout.fillHeight: true; color: theme.border }
                ColumnLayout {
                    Layout.fillWidth: true; Layout.fillHeight: true; spacing: 12
                    RowLayout {
                        Text { text: deck.selected.type || "Preview"; color: theme.foreground; font.family: theme.fontFamily; font.pixelSize: theme.subtitle; font.bold: true }
                        Item { Layout.fillWidth: true }
                        Button { text: deck.selected.pinned ? "Unpin Ctrl+P" : "Pin Ctrl+P"; enabled: !!deck.selected.id; onClicked: deck.pin_item(deck.selected.id, !deck.selected.pinned) }
                        Button { text: "Delete"; enabled: !!deck.selected.id; onClicked: deck.delete_item(deck.selected.id) }
                    }
                    Rectangle {
                        Layout.fillWidth: true; Layout.fillHeight: true; color: theme.surface; radius: theme.radius
                        ScrollView {
                            visible: deck.imageSource.length === 0
                            anchors.fill: parent; anchors.margins: 16; clip: true
                            TextArea { text: deck.preview || "Select a clip to preview"; readOnly: true; textFormat: TextEdit.PlainText; wrapMode: TextEdit.Wrap; color: theme.foreground; font.family: theme.codeFamily; font.pixelSize: theme.body; background: null; selectByMouse: true }
                        }
                        Image { anchors.fill: parent; anchors.margins: 16; visible: deck.imageSource.length > 0; source: deck.imageSource; fillMode: Image.PreserveAspectFit; asynchronous: true }
                    }
                    RowLayout {
                        Button { text: "Paste Enter"; enabled: !!deck.selected.id; onClicked: deck.paste_item(deck.selected.id); background: Rectangle { color: theme.accent; radius: 12 } }
                        Button { text: "Copy Ctrl+C"; enabled: !!deck.selected.id; onClicked: deck.copy_item(deck.selected.id) }
                        Button { text: "Transform"; enabled: !!deck.selected.id && deck.selected.type !== "image"; onClicked: transformMenu.open()
                            Menu { id: transformMenu
                                MenuItem { text: "Copy uppercase"; onTriggered: deck.copy_transformed(deck.selected.id, "uppercase") }
                                MenuItem { text: "Copy lowercase"; onTriggered: deck.copy_transformed(deck.selected.id, "lowercase") }
                                MenuItem { text: "Copy trimmed"; onTriggered: deck.copy_transformed(deck.selected.id, "trim") }
                            }
                        }
                        Item { Layout.fillWidth: true }
                        Button { text: "Load more"; visible: deck.hasMore; onClicked: deck.load_more() }
                    }
                }
            }
            RowLayout {
                visible: deck.message.length > 0
                Text { Layout.fillWidth: true; text: deck.message; textFormat: Text.PlainText; wrapMode: Text.Wrap; color: theme.accent; font.family: theme.fontFamily; font.pixelSize: theme.body }
                Button { text: "Retry connection"; onClicked: deck.retry_connection() }
            }
            Rectangle { Layout.fillWidth: true; height: 1; color: theme.border }
            Text { text: "Enter paste   Ctrl+C copy   Ctrl+P pin   Ctrl+L search   Esc dismiss"; color: theme.muted; font.family: theme.fontFamily; font.pixelSize: theme.caption }
        }
    }
    Timer { id: searchDelay; interval: 120; onTriggered: deck.search(search.text, categories.selected) }
}
