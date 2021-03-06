%global _version 2.0.0
%global _release 20200803.130531.git7619e1a0
Name:       iSulad-img
Version:    %{_version}
Release:    %{_release}
Summary:    a tool for downloading iSulad images


Group:      Applications/System
License:    Mulan PSL v2
URL:        https://gitee.com/src-openeuler/iSulad-img
Source0:    iSulad-img-2.0.tar.gz

BuildRequires:  golang >= 1.8.3
BuildRequires:  gpgme gpgme-devel
BuildRequires:  device-mapper-devel

%description
A tool for downloading iSulad images, written in go language


%prep
%setup -q -b 0 -c -n iSulad-img-%{version}

# apply the patchs
cp ./patch/* ./
cat series-patch.conf | while read line
do
        if [[ $line == '' || $line =~ ^\s*# ]]; then
                continue
        fi
        patch -p1 -F1 -s < $line
done
cd -

%build
make %{?_smp_mflags}

%install
install -d $RPM_BUILD_ROOT/%{_bindir}
install -m 0755 ./isulad-img %{buildroot}/%{_bindir}/isulad-img
install -d $RPM_BUILD_ROOT/%{_sysconfdir}/containers
install -m 0644 ./default-policy.json $RPM_BUILD_ROOT/%{_sysconfdir}/containers/policy.json

%clean
rm -rf %{buildroot}

%files
%defattr(-,root,root,-)
%{_bindir}/isulad-img
%{_sysconfdir}/*

%changelog
* Mon Aug 03 2020 openEuler Buildteam <buildteam@openeuler.org> - 2.0.0-20200803.130531.git7619e1a0
- Type:enhancement
- ID:NA
- SUG:NA
- DESC: add debug packages
