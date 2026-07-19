from types import MappingProxyType


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_proxy(da: dict[str, A], db: dict[str, B]):
    pa = MappingProxyType(da)
    pb = MappingProxyType(db)
    return pa["k"].execute() + pb["k"].run()


def use_proxy_inline(da: dict[str, A], db: dict[str, B]):
    return MappingProxyType(da)["k"].execute() + MappingProxyType(db)["k"].run()


def use_proxy_get(da: dict[str, A], db: dict[str, B]):
    return MappingProxyType(da).get("k").execute() + MappingProxyType(db).get("k").run()


def use_proxy_values(da: dict[str, A], db: dict[str, B]):
    return list(MappingProxyType(da).values())[0].execute() + list(MappingProxyType(db).values())[0].run()


def use_proxy_lit():
    return MappingProxyType({"k": A()})["k"].execute() + MappingProxyType({"k": B()})["k"].run()
