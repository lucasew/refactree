class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_list(da: dict[str, A], db: dict[str, B]) -> int:
    return list(da.items())[0][1].execute() + list(db.items())[0][1].run()


def use_tuple(da: dict[str, A], db: dict[str, B]) -> int:
    return tuple(da.items())[0][1].execute() + tuple(db.items())[0][1].run()


def use_assigned() -> int:
    da: dict[str, A] = {}
    db: dict[str, B] = {}
    return list(da.items())[0][1].execute() + list(db.items())[0][1].run()


def use_field_var(da: dict[str, A], db: dict[str, B]) -> int:
    a = list(da.items())[0][1]
    b = list(db.items())[0][1]
    return a.execute() + b.run()
