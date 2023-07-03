#ifndef _HOOKS_LINK_H_
#define _HOOKS_LINK_H_

#include "constants/syscall_macro.h"
#include "helpers/approvers.h"
#include "helpers/discarders.h"
#include "helpers/filesystem.h"
#include "helpers/syscalls.h"
#include "helpers/path_resolver.h"

int __attribute__((always_inline)) trace__sys_link(u8 async) {
    struct policy_t policy = fetch_policy(EVENT_LINK);
    if (is_discarded_by_process(policy.mode, EVENT_LINK)) {
        return 0;
    }

    struct syscall_cache_t syscall = {
        .type = EVENT_LINK,
        .policy = policy,
        .async = async,
    };

    cache_syscall(&syscall);

    return 0;
}

SYSCALL_KPROBE0(link) {
    return trace__sys_link(SYNC_SYSCALL);
}

SYSCALL_KPROBE0(linkat) {
    return trace__sys_link(SYNC_SYSCALL);
}

SEC("kprobe/do_linkat")
int kprobe_do_linkat(struct pt_regs *ctx) {
    struct syscall_cache_t* syscall = peek_syscall(EVENT_LINK);
    if (!syscall) {
        return trace__sys_link(ASYNC_SYSCALL);
    }
    return 0;
}

SEC("kprobe/vfs_link")
int kprobe_vfs_link(struct pt_regs *ctx) {
    struct syscall_cache_t *syscall = peek_syscall(EVENT_LINK);
    if (!syscall) {
        return 0;
    }

    if (syscall->link.target_dentry) {
        return 0;
    }

    struct dentry *src_dentry = (struct dentry *)PT_REGS_PARM1(ctx);
    syscall->link.src_dentry = src_dentry;

    syscall->link.target_dentry = (struct dentry *)PT_REGS_PARM3(ctx);
    // change the register based on the value of vfs_link_target_dentry_position
    if (get_vfs_link_target_dentry_position() == VFS_ARG_POSITION4) {
        // prevent the verifier from whining
        bpf_probe_read(&syscall->link.target_dentry, sizeof(syscall->link.target_dentry), &syscall->link.target_dentry);
        syscall->link.target_dentry = (struct dentry *) PT_REGS_PARM4(ctx);
    }

    // this is a hard link, source and target dentries are on the same filesystem & mount point
    // target_path was set by kprobe/filename_create before we reach this point.
    syscall->link.src_file.path_key.mount_id = get_path_mount_id(syscall->link.target_path);
    set_file_inode(src_dentry, &syscall->link.src_file, 0);

    if (filter_syscall(syscall, link_approvers)) {
        return mark_as_discarded(syscall);
    }

    fill_file_metadata(src_dentry, &syscall->link.src_file.metadata);
    syscall->link.target_file.metadata = syscall->link.src_file.metadata;

    // we generate a fake target key as the inode is the same
    syscall->link.target_file.path_key.ino = FAKE_INODE_MSW<<32 | bpf_get_prandom_u32();
    syscall->link.target_file.path_key.mount_id = syscall->link.src_file.path_key.mount_id;
    if (is_overlayfs(src_dentry)) {
        syscall->link.target_file.flags |= UPPER_LAYER;
    }

    syscall->resolver.dentry = src_dentry;
    syscall->resolver.key = syscall->link.src_file.path_key;
    syscall->resolver.discarder_type = syscall->policy.mode != NO_FILTER ? EVENT_LINK : 0;
    syscall->resolver.callback = PR_PROGKEY_CB_LINK_SRC_KPROBE;
    syscall->resolver.iteration = 0;
    syscall->resolver.ret = 0;

    resolve_path(ctx, DR_KPROBE);
    return 0;
}

SEC("kprobe/dr_link_src_callback")
int __attribute__((always_inline)) kprobe_dr_link_src_callback(struct pt_regs *ctx) {
    struct syscall_cache_t *syscall = peek_syscall(EVENT_LINK);
    if (!syscall) {
        return 0;
    }

    fill_path_ring_buffer_ref(&syscall->link.src_file.path_ref);

    if (syscall->resolver.ret == DENTRY_DISCARDED) {
        monitor_discarded(EVENT_LINK);
        return mark_as_discarded(syscall);
    }

    return 0;
}

int __attribute__((always_inline)) sys_link_ret(void *ctx, int retval, int dr_type) {
    if (IS_UNHANDLED_ERROR(retval)) {
        return 0;
    }

    struct syscall_cache_t *syscall = peek_syscall(EVENT_LINK);
    if (!syscall) {
        return 0;
    }

    int pass_to_userspace = !syscall->discarded && is_event_enabled(EVENT_LINK);

    // invalidate user space inode, so no need to bump the discarder revision in the event
    if (retval >= 0) {
        // for hardlink we need to invalidate the cache as the nlink counter in now > 1
        invalidate_inode(ctx, syscall->link.src_file.path_key.mount_id, syscall->link.src_file.path_key.ino, !pass_to_userspace);
    }

    if (pass_to_userspace) {
        syscall->resolver.dentry = syscall->link.target_dentry;
        syscall->resolver.key = syscall->link.target_file.path_key;
        syscall->resolver.discarder_type = 0;
        syscall->resolver.callback = PR_PROGKEY_CB_LINK_DST;
        syscall->resolver.iteration = 0;
        syscall->resolver.ret = 0;

        resolve_path(ctx, dr_type);
    }

    // if the tail call fails, we need to pop the syscall cache entry
    pop_syscall(EVENT_LINK);
    return 0;
}

SEC("kretprobe/do_linkat")
int kretprobe_do_linkat(struct pt_regs *ctx) {
    int retval = PT_REGS_RC(ctx);
    return sys_link_ret(ctx, retval, DR_KPROBE);
}

int __attribute__((always_inline)) kprobe_sys_link_ret(struct pt_regs *ctx) {
    int retval = PT_REGS_RC(ctx);
    return sys_link_ret(ctx, retval, DR_KPROBE);
}

SYSCALL_KRETPROBE(link) {
    return kprobe_sys_link_ret(ctx);
}

SYSCALL_KRETPROBE(linkat) {
    return kprobe_sys_link_ret(ctx);
}

SEC("tracepoint/handle_sys_link_exit")
int tracepoint_handle_sys_link_exit(struct tracepoint_raw_syscalls_sys_exit_t *args) {
    return sys_link_ret(args, args->ret, DR_TRACEPOINT);
}

int __attribute__((always_inline)) dr_link_dst_callback(void *ctx, int retval) {
    struct syscall_cache_t *syscall = pop_syscall(EVENT_LINK);
    if (!syscall) {
        return 0;
    }

    if (IS_UNHANDLED_ERROR(retval)) {
        return 0;
    }

    struct link_event_t event = {
        .event.type = EVENT_LINK,
        .event.timestamp = bpf_ktime_get_ns(),
        .syscall.retval = retval,
        .event.flags = syscall->async ? EVENT_FLAGS_ASYNC : 0,
        .source = syscall->link.src_file,
        .target = syscall->link.target_file,
    };

    struct proc_cache_t *entry = fill_process_context(&event.process);
    fill_container_context(entry, &event.container);
    fill_span_context(&event.span);
    fill_path_ring_buffer_ref(&event.target.path_ref);

    send_event(ctx, EVENT_LINK, event);

    return 0;
}

SEC("kprobe/dr_link_dst_callback")
int __attribute__((always_inline)) kprobe_dr_link_dst_callback(struct pt_regs *ctx) {
    int ret = PT_REGS_RC(ctx);
    return dr_link_dst_callback(ctx, ret);
}

SEC("tracepoint/dr_link_dst_callback")
int __attribute__((always_inline)) tracepoint_dr_link_dst_callback(struct tracepoint_syscalls_sys_exit_t *args) {
    return dr_link_dst_callback(args, args->ret);
}

#endif
