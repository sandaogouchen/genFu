create table if not exists conversation_sessions (
    id text primary key,
    user_id text,
    created_at text not null default CURRENT_TIMESTAMP,
    updated_at text not null default CURRENT_TIMESTAMP
);

create table if not exists conversation_messages (
    id integer primary key autoincrement,
    session_id text not null,
    role text not null,
    content text,
    payload text not null,
    created_at text not null default CURRENT_TIMESTAMP,
    foreign key(session_id) references conversation_sessions(id)
);

create index if not exists idx_conversation_messages_session_id on conversation_messages(session_id);
