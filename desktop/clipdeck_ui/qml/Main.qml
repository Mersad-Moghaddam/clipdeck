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
    property var selectedIds: []
    function toggleSelection(id) {
        let next = selectedIds.slice()
        let index = next.indexOf(id)
        if (index >= 0) next.splice(index, 1); else next.push(id)
        selectedIds = next
    }
    function age(value) {
        let seconds = Math.max(0, (Date.now() - Date.parse(value)) / 1000)
        if (!isFinite(seconds)) return ""
        if (seconds < 60) return "now"
        if (seconds < 3600) return Math.floor(seconds / 60) + "m"
        if (seconds < 86400) return Math.floor(seconds / 3600) + "h"
        return Math.floor(seconds / 86400) + "d"
    }
    flags: theme.frameless ? Qt.Window | Qt.FramelessWindowHint : Qt.Window
    onVisibleChanged: if (visible) requestActivate()

    Shortcut { sequence: "Escape"; onActivated: deck.dismiss() }
    Shortcut { sequence: "Ctrl+C"; onActivated: if (history.currentItem) deck.copy_item(history.currentItem.clip.id) }
    Shortcut { sequence: "Return"; onActivated: if (history.currentItem) deck.paste_item(history.currentItem.clip.id) }
    Shortcut { sequence: "Ctrl+P"; onActivated: if (history.currentItem) deck.pin_item(history.currentItem.clip.id, !history.currentItem.clip.pinned) }
    Shortcut { sequence: "Ctrl+L"; onActivated: search.forceActiveFocus() }
    Shortcut { sequence: "Ctrl+M"; onActivated: deck.merge_copy(window.selectedIds) }
    Shortcut { sequence: "Shift+Return"; onActivated: if (deck.selected.id) deck.paste_plain(deck.selected.id) }
    Shortcut { sequence: "Delete"; onActivated: if (deck.selected.id && !search.activeFocus) deck.delete_item(deck.selected.id) }
    Shortcut { sequence: "Shift+Delete"; onActivated: clearDialog.open() }
    Shortcut { sequence: "Ctrl+U"; onActivated: search.clear() }
    Shortcut { sequence: "Ctrl+T"; onActivated: if (deck.selected.id) transformMenu.open() }
    Shortcut { sequence: "Tab"; onActivated: window.nextCategory(1) }
    Shortcut { sequence: "Shift+Tab"; onActivated: window.nextCategory(-1) }
    Repeater {
        model: 9
        delegate: Item {
            required property int index
            Shortcut {
                sequence: "Alt+" + (index + 1)
                onActivated: if (index < deck.items.length) deck.paste_item(deck.items[index].id)
            }
        }
    }
    function nextCategory(step) {
        let names = ["All", "Pinned", "Colors", "Code", "Links", "Images", "Text"]
        categories.selected = names[(names.indexOf(categories.selected) + step + names.length) % names.length]
        deck.search(search.text, categories.selected)
    }
    Dialog {
        id: clearDialog
        width: Math.min(window.width - 40, 420)
        anchors.centerIn: parent
        modal: true
        title: "Clear unpinned history?"
        standardButtons: Dialog.Ok | Dialog.Cancel
        onAccepted: { window.selectedIds = []; deck.clear_history() }
        contentItem: Text { text: "Pinned clips are kept. This cannot be undone."; color: theme.foreground; font.family: theme.fontFamily }
        background: Rectangle { color: theme.surface; border.color: theme.accent; radius: theme.radius }
    }

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
                DeckButton { text: "×"; onClicked: deck.dismiss() }
            }
            RowLayout {
                id: categories
                property string selected: "All"
                spacing: 8
                Repeater {
                    model: ["All", "Pinned", "Colors", "Code", "Links", "Images", "Text"]
                    delegate: DeckButton {
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
                        color: history.currentIndex === index || window.selectedIds.indexOf(row.clip.id) >= 0 ? theme.selected : "transparent"
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
                            Text { text: window.age(row.clip.created_at); color: theme.muted; font.family: theme.fontFamily; font.pixelSize: theme.caption }
                        }
                        MouseArea { anchors.fill: parent; onClicked: function(mouse) { history.currentIndex = row.index; deck.select_item(row.clip); if (mouse.modifiers & Qt.ControlModifier) window.toggleSelection(row.clip.id) } onDoubleClicked: deck.copy_item(row.clip.id) }
                    }
                    ScrollBar.vertical: ScrollBar {}
                    Keys.onReturnPressed: if (currentItem) deck.paste_item(currentItem.clip.id)
                    Keys.onSpacePressed: if (currentItem) window.toggleSelection(currentItem.clip.id)
                }
                Rectangle { width: 1; Layout.fillHeight: true; color: theme.border }
                ColumnLayout {
                    Layout.fillWidth: true; Layout.fillHeight: true; spacing: 12
                    RowLayout {
                        Rectangle {
                            width: 34; height: 34; color: theme.surface; radius: 6
                            Text { anchors.centerIn: parent; text: (deck.selected.type || "TXT").slice(0,3).toUpperCase(); color: theme.foreground; font.family: theme.fontFamily; font.pixelSize: theme.bodySmall }
                        }
                        ColumnLayout {
                            spacing: 3
                            Text { text: deck.selected.type || "Preview"; color: theme.foreground; font.family: theme.fontFamily; font.pixelSize: theme.subtitle; font.bold: true }
                            Text { text: (deck.selected.source_app || "clipboard") + "  ·  " + window.age(deck.selected.created_at) + "  ·  " + (deck.selected.size_bytes || 0) + " B"; color: theme.muted; font.family: theme.fontFamily; font.pixelSize: theme.caption }
                        }
                        Item { Layout.fillWidth: true }
                        DeckButton { text: deck.selected.pinned ? "Unpin Ctrl+P" : "Pin Ctrl+P"; enabled: !!deck.selected.id; onClicked: deck.pin_item(deck.selected.id, !deck.selected.pinned) }
                        DeckButton { text: "Delete"; enabled: !!deck.selected.id; onClicked: deck.delete_item(deck.selected.id) }
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
                        DeckButton { text: "Paste Enter"; enabled: !!deck.selected.id; onClicked: deck.paste_item(deck.selected.id); primary: true }
                        DeckButton { text: "Plain Shift+Enter"; enabled: !!deck.selected.id; onClicked: deck.paste_plain(deck.selected.id) }
                        DeckButton { text: "Copy Ctrl+C"; enabled: !!deck.selected.id; onClicked: deck.copy_item(deck.selected.id) }
                        DeckButton { text: "Merge " + window.selectedIds.length; visible: window.selectedIds.length > 1; onClicked: deck.merge_copy(window.selectedIds) }
                        DeckButton { text: "Transform"; enabled: !!deck.selected.id && deck.selected.type !== "image"; onClicked: transformMenu.open()
                            Menu { id: transformMenu
                                MenuItem { text: "Copy uppercase"; onTriggered: deck.copy_transformed(deck.selected.id, "uppercase") }
                                MenuItem { text: "Copy lowercase"; onTriggered: deck.copy_transformed(deck.selected.id, "lowercase") }
                                MenuItem { text: "Copy trimmed"; onTriggered: deck.copy_transformed(deck.selected.id, "trim") }
                            }
                        }
                        Item { Layout.fillWidth: true }
                        DeckButton { text: "Load more"; visible: deck.hasMore; onClicked: deck.load_more() }
                    }
                }
            }
            RowLayout {
                visible: deck.message.length > 0
                Text { Layout.fillWidth: true; text: deck.message; textFormat: Text.PlainText; wrapMode: Text.Wrap; color: theme.accent; font.family: theme.fontFamily; font.pixelSize: theme.body }
                DeckButton { text: "Retry connection"; onClicked: deck.retry_connection() }
                DeckButton { text: "Start & enable at login"; onClicked: deck.enable_startup() }
            }
            Text {
                Layout.fillWidth: true
                visible: theme.captureWarning.length > 0
                text: theme.captureWarning
                textFormat: Text.PlainText
                wrapMode: Text.Wrap
                color: theme.foreground
                font.family: theme.fontFamily
                font.pixelSize: theme.body
            }
            Rectangle { Layout.fillWidth: true; height: 1; color: theme.border }
            Text { text: "Enter paste   Ctrl+C copy   Space select   Alt+1–9 quick   Ctrl+T transform   Ctrl+P pin   Shift+Del clear   Esc dismiss"; color: theme.muted; font.family: theme.fontFamily; font.pixelSize: theme.caption }
        }
    }
    Timer { id: searchDelay; interval: 120; onTriggered: deck.search(search.text, categories.selected) }
}
