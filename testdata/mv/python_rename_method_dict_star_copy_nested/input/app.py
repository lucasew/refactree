class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_star_sub(da: dict[str, list[A]], db: dict[str, list[B]]) -> int:
    return {**da}["k"][0].run() + {**db}["k"][0].run()


def use_star_assign(da: dict[str, list[A]], db: dict[str, list[B]]) -> int:
    ca = {**da}
    cb = {**db}
    return ca["k"][0].run() + cb["k"][0].run()


def use_star_for(da: dict[str, list[A]], db: dict[str, list[B]]) -> int:
    n = 0
    for a in {**da}["k"]:
        n += a.run()
    for b in {**db}["k"]:
        n += b.run()
    return n


def use_star_var(da: dict[str, list[A]], db: dict[str, list[B]]) -> int:
    ga = {**da}["k"]
    gb = {**db}["k"]
    return ga[0].run() + gb[0].run()


def use_star_values(da: dict[str, list[A]], db: dict[str, list[B]]) -> int:
    n = 0
    for ga in {**da}.values():
        n += ga[0].run()
    for gb in {**db}.values():
        n += gb[0].run()
    return n


def use_star_items(da: dict[str, list[A]], db: dict[str, list[B]]) -> int:
    n = 0
    for k, ga in {**da}.items():
        n += ga[0].run()
    for k, gb in {**db}.items():
        n += gb[0].run()
    return n


def use_star_get(da: dict[str, list[A]], db: dict[str, list[B]]) -> int:
    return {**da}.get("k")[0].run() + {**db}.get("k")[0].run()


def use_star_multi(da: dict[str, list[A]], ea: dict[str, list[A]], db: dict[str, list[B]]) -> int:
    return {**da, **ea}["k"][0].run() + {**db}["k"][0].run()


def use_star_with_pair(da: dict[str, list[A]], db: dict[str, list[B]]) -> int:
    return {**da, "j": [A()]}["j"][0].run() + {**db, "j": [B()]}["j"][0].run()


def use_copy_sub(da: dict[str, list[A]], db: dict[str, list[B]]) -> int:
    return da.copy()["k"][0].run() + db.copy()["k"][0].run()


def use_copy_assign(da: dict[str, list[A]], db: dict[str, list[B]]) -> int:
    ca = da.copy()
    cb = db.copy()
    return ca["k"][0].run() + cb["k"][0].run()


def use_copy_values(da: dict[str, list[A]], db: dict[str, list[B]]) -> int:
    n = 0
    for ga in da.copy().values():
        n += ga[0].run()
    for gb in db.copy().values():
        n += gb[0].run()
    return n


def use_unannotated_star():
    da = {"k": [A()]}
    db = {"k": [B()]}
    return {**da}["k"][0].run() + {**db}["k"][0].run()


def use_preserves_b(db: dict[str, list[B]]) -> int:
    return {**db}["k"][0].run()
