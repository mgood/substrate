package ignition

#var: {
  rebase_image: string
}

// https://coreos.github.io/butane/config-fcos-v1_5/
variant: "fcos"
version: "1.5.0"
passwd: users: [
  {
    name: "substrate"
    password_hash: "$y$j9T$zK4DDIlSx4fT3sjXMxklf.$JaBgIM8q9CXCcfgVa5ScYdp9/6Dg.wSk/dfYm3Uvo0B"
    groups: [
      "sudo",
      "wheel",
      "docker",
    ]
    ssh_authorized_keys_local: [
      "adamb-ssh-key.pub",
    ]
  }
]
storage: {
  disks: [
    {
      device: "/dev/disk/by-id/coreos-boot-disk"
      wipe_table: false
      partitions: [
        {
          number: 4
          label: "root"
          size_mib: 8192
          resize: true
        },
        {
          label: "var"
          size_mib: 0
        }
      ]
    }
  ]
  filesystems: [
    {
      path: "/var"
      device: "/dev/disk/by-partlabel/var"
      format: "btrfs"
      with_mount_unit: true
    }
  ]
  files: [
    // {
    //   path: "/etc/ostree/auth.json"
    //   mode: 0o600
    //   contents: {
    //     inline: """
    //        {
    //            "auths": {
    //                  "quay.io": {
    //                      "auth": "..."
    //                  }
    //            }
    //        }
    //        """
    //   }
    // }
  ]
}
systemd: {
  units: [
    {
      name: "getty@tty0.service"
      dropins: [
        {
          name: "autologin-substrate.conf"
          contents: """
            [Service]
            # Override Execstart in main unit
            ExecStart=
            # Add new Execstart with `-` prefix to ignore failure
            ExecStart=-/usr/sbin/agetty --autologin substrate --noclear %I $TERM
            TTYVTDisallocate=no
            """
        }
      ]
    },
    {
      name: "docker.service"
      mask: true
    },
    // {
    //   name: "serial-getty@ttyS0.service"
    //   dropins: {
    //     name: "autologin-substrate.conf"
    //     contents: """
    //       [Service]
    //       # Override Execstart in main unit
    //       ExecStart=
    //       # Add new Execstart with `-` prefix to ignore failure
    //       ExecStart=-/usr/sbin/agetty --autologin substrate --noclear %I $TERM
    //       TTYVTDisallocate=no
    //     """
    //   }
    // },
    {
      name: "substrateos-autorebase.service"
      enabled: true
      contents: """
        [Unit]
        Description=SubstrateOS autorebase to OCI and reboot
        ConditionFirstBoot=true
        After=network-online.target
        [Service]
        After=ignition-firstboot-complete.service
        Type=oneshot
        RemainAfterExit=yes
        ExecStart=rpm-ostree rebase --reboot ostree-unverified-registry:\(#var.rebase_image)
        [Install]
        WantedBy=multi-user.target
        """
    }
  ]
}