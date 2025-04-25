SELECT
    name AS overlay_name,
    REGEXP_EXTRACT(
    ARRAY_AGG(version ORDER BY version DESC)[SAFE_OFFSET(0)],
    r"^(\d+\.\d+).*")
    AS latest_major_version,
FROM
    (
    SELECT DISTINCT
        get_build_dependency_graph_request.sysroot.build_target.name,
        package.package_info.version
    FROM chromeos_ci_eng.analysis_event_log.last90days
    CROSS JOIN
        UNNEST(
        get_build_dependency_graph_response.dep_graph.package_deps)
        AS package
    WHERE
        get_build_dependency_graph_request IS NOT NULL
        AND package.package_info.category = 'sys-kernel'
        AND package.package_info.package_name LIKE '%chromeos-kernel%'
        AND package.package_info.version NOT LIKE '0.0%'
    ORDER BY version DESC
) GROUP BY name;