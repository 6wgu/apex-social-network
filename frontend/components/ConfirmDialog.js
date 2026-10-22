"use client";

import { createPortal } from "react-dom";
import Modal from "@/components/Modal";

// A "Are you sure?" dialog over a dimmed page. onConfirm runs when the red
// button is clicked; onClose when the dialog is cancelled or dismissed.
// Rendered in document.body so a scrolling or transformed parent (the feed)
// can never clip it.
export default function ConfirmDialog({ title, message, confirmLabel = "Delete", busy = false, onConfirm, onClose }) {
  return createPortal(
    <Modal title={title} onClose={busy ? () => {} : onClose}>
      <div className="confirm-dialog">
        <p className="confirm-message">{message}</p>
        <div className="confirm-actions">
          <button type="button" className="btn-secondary" onClick={onClose} disabled={busy}>
            Cancel
          </button>
          <button type="button" className="btn-danger" onClick={onConfirm} disabled={busy}>
            {busy ? "Deleting…" : confirmLabel}
          </button>
        </div>
      </div>
    </Modal>,
    document.body
  );
}
