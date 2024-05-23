import Adw from "gi://Adw?version=1";
import Gtk from "gi://Gtk?version=4.0";
import * as panrpc from "../../dist/index.js";
import Gio from "gi://Gio";
import Soup from "gi://Soup?version=3.0";

Gio._promisify(
  Soup.Session.prototype,
  "websocket_connect_async",
  "websocket_connect_finish"
);

const application = new Adw.Application({
  application_id: "com.github.pojntfx.panrpc.examples.websocket.client.gjs",
});

class Local {}

class Remote {}

application.connect("activate", () => {
  const window = new Adw.Window({
    application,
    default_width: 800,
    default_height: 600,
  });

  const toolbarView = new Adw.ToolbarView();

  const toolbar = new Adw.HeaderBar({
    show_title: false,
  });

  toolbarView.add_top_bar(toolbar);

  const statusPage = new Adw.StatusPage({
    title: "panrpc WebSocket Client Example (GJS)",
  });

  const section = new Gtk.Box({
    orientation: Gtk.Orientation.VERTICAL,
    halign: Gtk.Align.CENTER,
    spacing: 12,
  });

  const actions = new Gtk.Box({
    orientation: Gtk.Orientation.HORIZONTAL,
    halign: Gtk.Align.CENTER,
    spacing: 12,
  });

  const incrementButton = new Gtk.Button({
    label: "Increment remote counter by one",
  });

  actions.append(incrementButton);

  const decrementButton = new Gtk.Button({
    label: "Decrement remote counter by one",
  });

  actions.append(decrementButton);

  section.append(actions);

  const frame = new Gtk.Frame({
    margin_start: 12,
    margin_end: 12,
    margin_bottom: 12,
  });

  const scrolledWindow = new Gtk.ScrolledWindow({
    widthRequest: 320,
  });

  const textView = new Gtk.TextView({
    left_margin: 12,
    right_margin: 12,
    top_margin: 12,
    bottom_margin: 12,
    editable: false,
    cursor_visible: false,
  });

  textView.buffer.set_text("No logs yet", -1);

  scrolledWindow.set_child(textView);

  frame.set_child(scrolledWindow);

  section.append(frame);

  statusPage.set_child(section);

  toolbarView.set_property("content", statusPage);

  window.set_property("content", toolbarView);

  window.show();

  let clients = 0;

  const registry = new panrpc.Registry(
    new Local(),
    new Remote(),

    {
      onClientConnect: () => {
        clients++;

        console.error(clients, "clients connected");
      },
      onClientDisconnect: () => {
        clients--;

        console.error(clients, "clients connected");
      },
    }
  );

  console.log(registry);
});

application.run([]);
