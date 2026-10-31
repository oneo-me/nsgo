// The host of the app's own web page, which nsgo.yaml's web.host names:
// it answers the app's "time" calls, logs its "tapped" posts, takes its
// order links and greets it with a message as it starts.
export function createHost(player) {
  player.hostEvent("message", { text: "Hello from the page" });
  return {
    call(name) {
      if (name === "time") return new Date().toLocaleTimeString();
      throw new Error(`no host function ${name}`);
    },
    post(name, args) {
      console.log(`the app posted ${name}`, args);
    },
    openURL(url) {
      return url.startsWith("https://example.com/orders/");
    },
  };
}
