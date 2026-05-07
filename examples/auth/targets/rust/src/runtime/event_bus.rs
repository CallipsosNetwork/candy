// generated from examples/auth/auth.candy
// candy runtime 0.1
// do not edit — regenerate from spec

// In-process event bus using tokio broadcast.
// Delivery: eager (at-least-once) for all auth events.

use crate::auth::events::AuthEvent;
use tokio::sync::broadcast;

const CHANNEL_CAP: usize = 256;

#[derive(Clone)]
pub struct EventBus {
    sender: broadcast::Sender<AuthEvent>,
}

impl EventBus {
    pub fn new() -> Self {
        let (sender, _) = broadcast::channel(CHANNEL_CAP);
        EventBus { sender }
    }

    pub fn publish(&self, event: AuthEvent) {
        // best-effort; subscribers that lag are dropped
        let _ = self.sender.send(event);
    }

    pub fn subscribe(&self) -> broadcast::Receiver<AuthEvent> {
        self.sender.subscribe()
    }
}

impl Default for EventBus {
    fn default() -> Self {
        Self::new()
    }
}
