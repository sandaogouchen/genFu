create table if not exists conversation_session_memories (
    session_id text primary key,
    summary text not null default '',
    last_intent text not null default '',
    created_at text not null default CURRENT_TIMESTAMP,
    updated_at text not null default CURRENT_TIMESTAMP,
    foreign key(session_id) references conversation_sessions(id)
);

create index if not exists idx_conversation_session_memories_updated_at
    on conversation_session_memories(updated_at desc);
