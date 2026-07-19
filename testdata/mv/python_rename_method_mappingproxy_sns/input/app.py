from types import MappingProxyType, SimpleNamespace


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_proxy_get():
    return (
        MappingProxyType(vars(SimpleNamespace(k=A())))["k"].run()
        + MappingProxyType(vars(SimpleNamespace(k=B())))["k"].run()
    )


def use_proxy_values():
    return (
        next(iter(MappingProxyType(vars(SimpleNamespace(k=A()))).values())).run()
        + next(iter(MappingProxyType(vars(SimpleNamespace(k=B()))).values())).run()
    )


def use_proxy_dunder():
    return (
        MappingProxyType(SimpleNamespace(k=A()).__dict__)["k"].run()
        + MappingProxyType(SimpleNamespace(k=B()).__dict__)["k"].run()
    )


def use_preserves_b():
    return MappingProxyType(vars(SimpleNamespace(k=B())))["k"].run()
