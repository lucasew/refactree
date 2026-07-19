from collections import ChainMap


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_maps_sub(da: dict[str, A], db: dict[str, B]):
    return ChainMap(da).maps[0]["k"].execute() + ChainMap(db).maps[0]["k"].run()


def use_maps_assign(da: dict[str, A], db: dict[str, B]):
    ma = ChainMap(da).maps[0]
    mb = ChainMap(db).maps[0]
    return ma["k"].execute() + mb["k"].run()


def use_maps_local(da: dict[str, A], db: dict[str, B]):
    ca = ChainMap(da)
    cb = ChainMap(db)
    return ca.maps[0]["k"].execute() + cb.maps[0]["k"].run()


def use_maps_lit():
    return (
        ChainMap({"k": A()}).maps[0]["k"].execute()
        + ChainMap({"k": B()}).maps[0]["k"].run()
    )


def use_maps_get(da: dict[str, A], db: dict[str, B]):
    return ChainMap(da).maps[0].get("k").execute() + ChainMap(db).maps[0].get("k").run()


def use_maps_values(da: dict[str, A], db: dict[str, B]):
    return (
        list(ChainMap(da).maps[0].values())[0].execute()
        + list(ChainMap(db).maps[0].values())[0].run()
    )


def use_parents(da: dict[str, A], ea: dict[str, A], db: dict[str, B], eb: dict[str, B]):
    return (
        ChainMap(da, ea).parents["k"].execute()
        + ChainMap(db, eb).parents["k"].run()
    )


def use_parents_assign(da: dict[str, A], ea: dict[str, A], db: dict[str, B], eb: dict[str, B]):
    pa = ChainMap(da, ea).parents
    pb = ChainMap(db, eb).parents
    return pa["k"].execute() + pb["k"].run()
