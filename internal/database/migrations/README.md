Numbered golang-migrate files, embedded into the binary: `NNNNNN_name.up.sql` and `NNNNNN_name.down.sql`.
Each phase adds the tables it uses (architecture §3, §15); a dropped feature gets its own migration that removes them.
