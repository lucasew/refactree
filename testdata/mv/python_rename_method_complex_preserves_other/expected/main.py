class Box:
    def assist(self):
        return 1


class Other:
    def helper(self):
        return 9


def use(xs, obj):
    return xs[0].helper() + obj.box.helper() + Other.helper(Other())
