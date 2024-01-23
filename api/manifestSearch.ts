import { VercelRequest, VercelResponse } from "@vercel/node";
import { createKysely } from "@vercel/postgres-kysely";

export default async function handler(req: VercelRequest, res: VercelResponse) {
  // only allow POST requests
  if (req.method !== "POST") {
    res.status(405).send("Method Not Allowed");
    return;
  }

  // Parse the request body as JSON
  const { Query, Inclusions, Filters } = req.body as {
    Query: SearchRequestMatch;
    Inclusions: SearchRequestInclusionAndFilterSchema;
    Filters: SearchRequestInclusionAndFilterSchema;
  };
  console.log(JSON.stringify(req.body, null, 0));
  console.log(`Query: ${JSON.stringify(Query, null, 0)}`);
  console.log(`Inclusions: ${JSON.stringify(Inclusions, null, 0)}`);
  console.log(`Filters: ${JSON.stringify(Filters, null, 0)}`);

  const db = createKysely<Database>({
    connectionString: process.env.DATABASE_URL,
  });
  const query = db.selectFrom("packages").selectAll();

  if (Query.MatchType === "Exact") {
    query.where("PackageIdentifier", "=", Query.KeyWord);
  } else if (Query.MatchType === "CaseInsensitive") {
    query.where("PackageIdentifier", "ilike", Query.KeyWord);
  } else if (Query.MatchType === "StartsWith") {
    query.where("PackageIdentifier", "ilike", `${Query.KeyWord}%`);
  } else {
    query.where("PackageIdentifier", "ilike", `%${Query.KeyWord}%`);
  }

  // combine both Inclusions and Filters into a single array
  let inclusionsAndFilters: SearchRequestInclusionAndFilterSchema = [];
  if (Inclusions) {
    inclusionsAndFilters = inclusionsAndFilters.concat(Inclusions);
  }
  if (Filters) {
    inclusionsAndFilters = inclusionsAndFilters.concat(Filters);
  }

  // apply each inclusion and filter to the query
  for (const inclusionOrFilter of inclusionsAndFilters) {
    if (
      inclusionOrFilter.PackageMatchField ===
      "NormalizedPackageNameAndPublisher"
    ) {
      query.where((eb) =>
        eb.or([
          eb(
            "PackageName",
            "ilike",
            `%${inclusionOrFilter.RequestMatch.KeyWord}%`
          ),
          eb(
            "Publisher",
            "ilike",
            `%${inclusionOrFilter.RequestMatch.KeyWord}%`
          ),
        ])
      );
    } else if (inclusionOrFilter.PackageMatchField !== "Market") {
      switch (inclusionOrFilter.PackageMatchField) {
        case "Command":
          // note: it is "Commands" not "Command"
          query.where("Commands", "@>", [
            inclusionOrFilter.RequestMatch.KeyWord,
          ]);
          break;
        case "Tag":
          // note: it is "Tags" not "Tag"
          query.where("Tags", "@>", [inclusionOrFilter.RequestMatch.KeyWord]);
          break;
        case "PackageFamilyName":
          query.where("PackageFamilyName", "@>", [
            inclusionOrFilter.RequestMatch.KeyWord,
          ]);
          break;
        case "ProductCode":
          query.where("ProductCode", "@>", [
            inclusionOrFilter.RequestMatch.KeyWord,
          ]);
          break;
        default:
          if (inclusionOrFilter.RequestMatch.MatchType === "Exact") {
            query.where(
              inclusionOrFilter.PackageMatchField,
              "=",
              inclusionOrFilter.RequestMatch.KeyWord
            );
          } else if (
            inclusionOrFilter.RequestMatch.MatchType === "CaseInsensitive"
          ) {
            query.where(
              inclusionOrFilter.PackageMatchField,
              "ilike",
              inclusionOrFilter.RequestMatch.KeyWord
            );
          } else if (
            inclusionOrFilter.RequestMatch.MatchType === "StartsWith"
          ) {
            query.where(
              inclusionOrFilter.PackageMatchField,
              "ilike",
              `${inclusionOrFilter.RequestMatch.KeyWord}%`
            );
          } else {
            query.where(
              inclusionOrFilter.PackageMatchField,
              "ilike",
              `%${inclusionOrFilter.RequestMatch.KeyWord}%`
            );
          }
          break;
      }
    }
  }

  if (req.body.MaxiumumResults) {
    query.limit(req.body.MaxiumumResults);
  }

  // execute the query and return the results
  const results = await query.execute();

  let data: {}[] = [];
  for (const result of results) {
    data.push({
      PackageIdentifier: result.PackageIdentifier,
      PackageName: result.PackageName,
      Publisher: result.Publisher,
      Versions: [
        {
          PackageVersion: result.PackageVersion,
          // note: it is "PackageFamilyNames" not "PackageFamilyName
          PackageFamilyNames: result.PackageFamilyName,
          // note: it is "ProductCodes" not "ProductCode"
          ProductCodes: result.ProductCode,
        },
      ],
    });
  }

  res.status(200).json({
    Data: data,
    UnsupportedPackageMatchFields: ["Market"],
  });
}

interface Database {
  packages: PackagesTable;
}

interface PackagesTable {
  PackageIdentifier: string;
  PackageVersion: string;
  PackageName: string;
  Publisher: string;
  Moniker: string;
  ProductCode: string[];
  Commands: string[];
  Tags: string[];
  PackageFamilyName: string[];
}

type SearchRequestInclusionAndFilterSchema = Array<{
  PackageMatchField:
    | "PackageIdentifier"
    | "PackageName"
    | "Moniker"
    | "Command"
    | "Tag"
    | "PackageFamilyName"
    | "ProductCode"
    | "NormalizedPackageNameAndPublisher"
    | "Market";
  RequestMatch: SearchRequestMatch;
}>;

type SearchRequestMatch = {
  KeyWord: string;
  MatchType:
    | "Exact"
    | "CaseInsensitive"
    | "StartsWith"
    | "Substring"
    | "Wildcard"
    | "Fuzzy"
    | "FuzzySubstring";
};
