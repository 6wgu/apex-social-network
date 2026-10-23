"use client";

import { useState } from "react";
import Icon from "@/components/Icon";
import Modal from "@/components/Modal";
import PostComposer from "@/components/PostComposer";
import { notify } from "@/components/Toaster";

export default function NewPostButton({ onCreated, groupId }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button type="button" className="btn-primary new-post-btn" onClick={() => setOpen(true)}>
        <Icon name="plus" size={18} />
        New Post
      </button>
      {open && (
        <Modal title="Create post" onClose={() => setOpen(false)}>
          <PostComposer
            groupId={groupId}
            onCreated={(post) => {
              onCreated(post);
              setOpen(false);
              notify({ tone: "success", message: "Your post is live." });
            }}
          />
        </Modal>
      )}
    </>
  );
}
