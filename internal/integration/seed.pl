use strict;
use warnings;
use App::Netdisco;
use Dancer ':script';
use Dancer::Plugin::DBIC;
use Dancer::Plugin::Passphrase;
use POSIX qw(strftime);

my $now = strftime('%Y-%m-%d %H:%M:%S', gmtime);

for my $tenant ('netdisco', 'lab') {
    my $schema = schema($tenant);
    $schema->resultset('Session')->update_or_create({
        id => 'dancer_session_cookie_key',
        a_session => 'integration-only-cookie-key-not-a-production-secret',
    });
    $schema->resultset('User')->update_or_create({
        username => 'integration-admin',
        password => passphrase('integration-password')->generate,
        admin => 1, port_control => 1,
        token => 'integration-admin-token', token_from => time,
        token_no_expire => 1,
    });
    $schema->resultset('User')->update_or_create({
        username => 'integration-reader',
        password => passphrase('integration-password')->generate,
        admin => 0, port_control => 0,
        token => 'integration-reader-token', token_from => time,
        token_no_expire => 1,
    });
    $schema->resultset('User')->update_or_create({
        username => 'integration-login',
        password => passphrase('integration-password')->generate,
        admin => 0, port_control => 0,
    });
    $schema->resultset('Device')->update_or_create({
        ip => '192.0.2.10', dns => 'switch-a.invalid',
        name => ($tenant eq 'lab' ? 'tenant-switch' : 'switch-a'),
        vendor => 'fixture-vendor', model => 'fixture-switch', os => 'fixture-os',
        os_ver => '1.0', serial => 'fixture-a', layers => '01100000',
        last_discover => $now, last_macsuck => $now, creation => $now,
        location => 'integration-lab', mac => '02:00:00:00:10:00',
        custom_fields => '{"owner":"integration"}', tags => ['fixture'],
    });
    next if $tenant eq 'lab';

    $schema->resultset('Device')->update_or_create({
        ip => '192.0.2.11', dns => 'switch-b.invalid', name => 'switch-b',
        vendor => 'fixture-vendor', model => 'fixture-switch', layers => '01100000',
        location => 'integration-lab', mac => '02:00:00:00:11:00',
        last_discover => $now, creation => $now,
    });
    for my $ip ('192.0.2.10', '192.0.2.11') {
        $schema->resultset('DeviceIp')->update_or_create({
            ip => $ip, alias => $ip, subnet => '192.0.2.0/24',
            port => 'Gi1/0/1', dns => 'switch-a.invalid',
        });
        $schema->resultset('DeviceVlan')->update_or_create({
            ip => $ip, vlan => 100, description => 'fixture-vlan', last_discover => $now,
        });
    }
    for my $port ('Gi1/0/1', 'Gi1/0/2', 'Port-channel1', 'Radio0') {
        $schema->resultset('DevicePort')->update_or_create({
            ip => '192.0.2.10', port => $port, name => 'fixture-port',
            type => ($port eq 'Radio0' ? 'ieee80211' : 'ethernetCsmacd'),
            up => 'up', up_admin => 'up', duplex => 'full', duplex_admin => 'full',
            speed => '1.0 G', speed_admin => '1.0 G', mtu => 1500,
            vlan => '100', pvid => 100, stp => 'forwarding',
            is_uplink => ($port eq 'Gi1/0/2' ? 1 : 0),
            is_master => ($port eq 'Port-channel1' ? 1 : 0),
            slave_of => ($port eq 'Gi1/0/1' ? 'Port-channel1' : undef),
            remote_ip => ($port eq 'Gi1/0/2' ? '192.0.2.11' : undef),
            remote_port => ($port eq 'Gi1/0/2' ? 'Gi1/0/1' : undef),
            custom_fields => '{"owner":"integration"}', tags => ['fixture'],
        });
        $schema->resultset('DevicePortVlan')->update_or_create({
            ip => '192.0.2.10', port => $port, vlan => 100,
            native => 1, egress_tag => 0, vlantype => 'static', last_discover => $now,
        });
        $schema->resultset('DevicePortProperties')->update_or_create({
            ip => '192.0.2.10', port => $port, ifindex => 101,
            raw_speed => 1_000_000_000, faststart => 1,
            remote_is_wap => ($port eq 'Radio0' ? 1 : 0),
        });
    }
    $schema->resultset('DevicePort')->update_or_create({
        ip => '192.0.2.11', port => 'Gi1/0/1', up => 'up', up_admin => 'up',
        remote_ip => '192.0.2.10', remote_port => 'Gi1/0/2',
        vlan => '200', duplex => 'half', is_uplink => 1,
    });
    $schema->resultset('DeviceModule')->update_or_create({
        ip => '192.0.2.10', index => 1, name => 'fixture-module',
        description => 'Synthetic module', class => 'module', fru => 1,
        last_discover => $now,
    });
    $schema->resultset('DevicePower')->update_or_create({
        ip => '192.0.2.10', module => 1, power => 60000, status => 'on',
    });
    $schema->resultset('DevicePortPower')->update_or_create({
        ip => '192.0.2.10', port => 'Gi1/0/1', module => 1,
        admin => 'auto', status => 'deliveringPower', class => 'class3', power => 5000,
    });
    $schema->resultset('DevicePortSsid')->update_or_create({
        ip => '192.0.2.10', port => 'Radio0', ssid => 'fixture-wifi',
        broadcast => 1, bssid => '02:00:00:00:10:01',
    });
    $schema->resultset('DevicePortWireless')->update_or_create({
        ip => '192.0.2.10', port => 'Radio0', channel => 36, power => 12,
    });
    $schema->resultset('DevicePortLog')->update_or_create({
        id => 1, ip => '192.0.2.10', port => 'Gi1/0/1',
        reason => 'fixture', log => 'Synthetic port log', action => 'portcontrol',
        username => 'integration-admin', userip => '127.0.0.1',
    });
    $schema->resultset('Oui')->update_or_create({
        oui => '02:00:00', company => 'Synthetic Devices', abbrev => 'Fixture',
    });
    for my $suffix (1, 2) {
        my $mac = "02:00:00:00:00:0$suffix";
        $schema->resultset('Node')->update_or_create({
            mac => $mac, switch => '192.0.2.10', port => 'Gi1/0/1',
            vlan => '100', active => ($suffix == 1 ? 1 : 0), oui => '02:00:00',
            time_first => $now, time_recent => $now, time_last => $now,
        });
        $schema->resultset('NodeIp')->update_or_create({
            mac => $mac, ip => "198.51.100.$suffix", vrf => '', active => ($suffix == 1 ? 1 : 0),
            dns => "node-$suffix.invalid", time_first => $now, time_last => $now,
            seen_on_router_first => '{"192.0.2.10":"2026-01-01 00:00:00"}',
            seen_on_router_last => '{"192.0.2.10":"2026-01-01 00:00:00"}',
        });
    }
    $schema->resultset('NodeNbt')->update_or_create({
        mac => '02:00:00:00:00:01', ip => '198.51.100.1', nbname => 'fixture-node',
        domain => 'FIXTURE', nbuser => 'synthetic', server => 0, active => 1,
    });
    $schema->resultset('NodeWireless')->update_or_create({
        mac => '02:00:00:00:00:01', ssid => 'fixture-wifi',
        uptime => 3600, sigstrength => -42, txrate => 144, maxrate => 300,
    });
    $schema->resultset('Node')->update_or_create({
        mac => '02:00:00:00:00:03', switch => '192.0.2.10', port => 'Radio0',
        vlan => '100', active => 1, oui => '02:00:00',
        time_first => $now, time_recent => $now, time_last => $now,
    });
    $schema->resultset('Statistics')->update_or_create({
        day => substr($now, 0, 10), device_count => 2, device_ip_count => 2,
        device_link_count => 1, device_port_count => 5, device_port_up_count => 5,
        ip_table_count => 2, ip_active_count => 1, node_table_count => 3,
        node_active_count => 2, netdisco_ver => '2.117002', schema_ver => '104',
    });
}
print "seed ready\n";
