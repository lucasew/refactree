class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_star_copy_annotated(da: dict[str, A], db: dict[str, B]):
    return {**da}["k"].run() + {**db}["k"].run()


def use_star_copy_assign(da: dict[str, A], db: dict[str, B]):
    ca = {**da}
    cb = {**db}
    return ca["k"].run() + cb["k"].run()


def use_star_copy_unannotated():
    da = {}
    db = {}
    da["k"] = A()
    db["k"] = B()
    return {**da}["k"].run() + {**db}["k"].run()


def use_star_copy_with_pair(da: dict[str, A], db: dict[str, B]):
    return {**da, "j": A()}["j"].run() + {**db, "j": B()}["j"].run()


def use_star_copy_multi(da: dict[str, A], ea: dict[str, A], db: dict[str, B]):
    return {**da, **ea}["k"].run() + {**db}["k"].run()


def use_star_copy_get(da: dict[str, A], db: dict[str, B]):
    return {**da}.get("k").run() + {**db}.get("k").run()


def use_star_copy_values(da: dict[str, A], db: dict[str, B]):
    return list({**da}.values())[0].run() + list({**db}.values())[0].run()


def use_preserves_b(db: dict[str, B]):
    return {**db}["k"].run()
